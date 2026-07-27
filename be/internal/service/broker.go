package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/crypto"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

type TossRegistry struct {
	cfg        config.TossConfig
	global     *toss.Client
	allowGlobal bool
	creds      *repository.BrokerCredentialsRepo
	box        *crypto.Box
	redis      *redis.Client

	mu      sync.Mutex
	byUser  map[int64]*toss.Client
}

func NewTossRegistry(
	cfg config.TossConfig,
	global *toss.Client,
	allowGlobal bool,
	creds *repository.BrokerCredentialsRepo,
	box *crypto.Box,
	redis *redis.Client,
) *TossRegistry {
	return &TossRegistry{
		cfg:         cfg,
		global:      global,
		allowGlobal: allowGlobal,
		creds:       creds,
		box:         box,
		redis:       redis,
		byUser:      make(map[int64]*toss.Client),
	}
}

func (r *TossRegistry) ClientForUser(ctx context.Context, userID int64) (*toss.Client, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user id")
	}

	r.mu.Lock()
	if c, ok := r.byUser[userID]; ok && c != nil && c.Enabled() {
		r.mu.Unlock()
		return c, nil
	}
	r.mu.Unlock()

	cred, err := r.creds.GetActive(ctx, userID, "toss")
	if err != nil {
		if r.allowGlobal && r.global != nil && r.global.Enabled() {
			return r.global, nil
		}
		return nil, repository.ErrBrokerNotLinked
	}

	clientID, err := r.box.Decrypt(cred.ClientIDEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt client id: %w", err)
	}
	clientSecret, err := r.box.Decrypt(cred.ClientSecretEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt client secret: %w", err)
	}

	tokenKey := fmt.Sprintf("toss:oauth:user:%d", userID)
	client, err := toss.NewClientForCredentials(
		r.cfg.BaseURL,
		clientID,
		clientSecret,
		toss.NewTokenStore(r.redis),
		tokenKey,
	)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.byUser[userID] = client
	r.mu.Unlock()
	return client, nil
}

func (r *TossRegistry) InvalidateUser(userID int64) {
	r.mu.Lock()
	delete(r.byUser, userID)
	r.mu.Unlock()
}

func (r *TossRegistry) GlobalClient() *toss.Client {
	return r.global
}

type BrokerService struct {
	creds    *repository.BrokerCredentialsRepo
	accounts *repository.BrokerAccountRepo
	box      *crypto.Box
	tossReg  *TossRegistry
}

func NewBrokerService(
	creds *repository.BrokerCredentialsRepo,
	accounts *repository.BrokerAccountRepo,
	box *crypto.Box,
	tossReg *TossRegistry,
) *BrokerService {
	return &BrokerService{creds: creds, accounts: accounts, box: box, tossReg: tossReg}
}

func (s *BrokerService) LinkToss(ctx context.Context, userID int64, clientID, clientSecret string) (*model.BrokerLinkStatus, error) {
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("client id and secret are required")
	}

	tokenKey := fmt.Sprintf("toss:oauth:user:%d:verify", userID)
	client, err := toss.NewClientForCredentials(
		s.tossReg.cfg.BaseURL,
		clientID,
		clientSecret,
		toss.NewTokenStore(s.tossReg.redis),
		tokenKey,
	)
	if err != nil {
		return nil, err
	}

	tossAccounts, err := client.GetAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("toss verification failed: %w", err)
	}

	idEnc, err := s.box.Encrypt(clientID)
	if err != nil {
		return nil, err
	}
	secretEnc, err := s.box.Encrypt(clientSecret)
	if err != nil {
		return nil, err
	}
	hint := clientID
	if len(hint) > 6 {
		hint = hint[:3] + "…" + hint[len(hint)-2:]
	}
	if err := s.creds.Upsert(ctx, userID, "toss", idEnc, secretEnc, hint); err != nil {
		return nil, err
	}

	s.tossReg.InvalidateUser(userID)

	synced := make([]model.BrokerAccount, 0, len(tossAccounts))
	for i, acct := range tossAccounts {
		synced = append(synced, model.BrokerAccount{
			UserID:      userID,
			BrokerCode:  "toss",
			AccountName: accountDisplayName(acct),
			AccountSeq:  acct.AccountSeq,
			AccountNo:   acct.AccountNo,
			AccountType: string(acct.AccountType),
			IsPrimary:   i == 0,
		})
	}
	if err := s.accounts.ReplaceForUser(ctx, userID, "toss", synced); err != nil {
		return nil, err
	}

	return s.Status(ctx, userID)
}

func (s *BrokerService) UnlinkToss(ctx context.Context, userID int64) error {
	if err := s.creds.Deactivate(ctx, userID, "toss"); err != nil {
		return err
	}
	s.tossReg.InvalidateUser(userID)
	return s.accounts.DeleteForUser(ctx, userID, "toss")
}

func (s *BrokerService) Status(ctx context.Context, userID int64) (*model.BrokerLinkStatus, error) {
	linked, hint, linkedAt, err := s.creds.IsLinked(ctx, userID, "toss")
	if err != nil {
		return nil, err
	}
	if !linked {
		return &model.BrokerLinkStatus{Linked: false, BrokerCode: "toss"}, nil
	}

	accts, err := s.accounts.ListByUser(ctx, userID, "toss")
	if err != nil {
		return nil, err
	}

	status := &model.BrokerLinkStatus{
		Linked:       true,
		BrokerCode:   "toss",
		ClientIDHint: hint,
		Accounts:     accts,
	}
	if !linkedAt.IsZero() {
		status.LinkedAt = linkedAt.Format(time.RFC3339)
	}
	return status, nil
}
