# Docker 배포 (API + Tesseract)

Quant Port API는 **항상 Docker**로 실행합니다. 이미지에 Tesseract OCR(`kor+eng`)이 포함되어 클라우드·로컬 환경이 동일합니다.

## 구성

| 구성요소 | 설명 |
| --- | --- |
| `be/Dockerfile` | multi-stage build → `debian:bookworm-slim` + `tesseract-ocr` |
| `docker-compose.yml` | `redis` + `api` (기본 `up`에 포함) |
| `be/scripts/restart.ps1` | `docker compose up -d --build` + health + **Tesseract 검증** |
| `IMPORT_TEMP_DIR` | `/var/lib/quant-port/import` (컨테이너 내부) |

## 로컬 실행 (Windows)

```powershell
cd be
cp .env.example .env   # JWT_SECRET 등 설정
.\scripts\restart.ps1 -Background
```

로그: `docker logs -f quant-port-api`

Tesseract 정상:

```
import: tesseract ready (langs=kor+eng, temp=/var/lib/quant-port/import)
```

MySQL은 호스트 `3306` → `host.docker.internal` (compose `extra_hosts` 포함).

## tmpfs를 쓰지 않는 이유

초기 설계는 import 경로에 **tmpfs 256MB**를 마운트했으나:

- tmpfs는 **RAM**을 사용해 소형 VM/노트북에서 메모리 압박·OOM 가능
- 256MB 상한은 job 10장 + crop/OCR 중간 파일 peak에 **부족할 수 있음**
- 스크린샷 import는 **단기·소량**이라 컨테이너 writable layer(디스크)로 충분

현재는 **컨테이너 내부 디스크**(`/var/lib/quant-port/import`)만 사용합니다. 볼륨 bind mount 없음 → **컨테이너 재생성 시 디렉터리 전체 소거**.

## 스크린샷 임시 파일 — 삭제 로직

경로: `/var/lib/quant-port/import/{jobId}/`

```
img_00.jpg      업로드 원본
crop_00.png     crop
ocr_00.png      OCR 전처리
```

| 트리거 | 코드 | 동작 |
| --- | --- | --- |
| OCR·preview 완료 | `processJob` 마지막 | `os.RemoveAll(tempRoot/jobId)` |
| commit 완료 | `CommitJob` | `os.RemoveAll(tempRoot/jobId)` |
| 업로드 실패 | `StartScreenshotsImport` | jobDir 생성 전/실패 시 `RemoveAll` |
| 24h+ orphan | `runOrphanSweeper` (6h tick) | `ModTime` 24h 지난 job 폴더 삭제 |
| 컨테이너 replace | Docker | writable layer 폐기 → 잔여 없음 |

Job **상태·preview**는 Redis (`import:holdings:job:{id}`, TTL 24h)에만 남고, **이미지 파일은 Redis에 저장하지 않음**.

## 클라우드

1. `be/Dockerfile` 이미지 push (ECR/GCR 등)
2. env: `MYSQL_DSN`, `REDIS_URL`, `JWT_SECRET`, `IMPORT_TEMP_DIR=/var/lib/quant-port/import`
3. import 경로: **emptyDir**(디스크) 권장 — tmpfs(메모리)는 선택 사항
4. Tesseract는 이미지 내장 — 노드 추가 설치 불필요

## 환경 변수

| 변수 | Docker 기본값 |
| --- | --- |
| `IMPORT_TEMP_DIR` | `/var/lib/quant-port/import` |
| `REDIS_URL` | `redis://redis:6379/0` (compose override) |
