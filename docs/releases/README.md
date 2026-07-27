# Quant Port — Release Notes

| Version | Date | Document |
|---------|------|----------|
| **v0.1** | 2026-07-27 | [v0.1.md](./v0.1.md) · tag `v0.1` |

## v0.1으로 복원하기

Git 태그 `v0.1`이 이 스냅샷을 가리킵니다.

```powershell
# 작업 중 변경사항 확인 후
git status

# v0.1 코드 상태로 되돌리기 (브랜치 유지)
git restore --source v0.1 .

# 또는 해당 커밋으로 hard reset (로컬 변경 전부 폐기 — 주의)
git reset --hard v0.1
```

에이전트에게 **「v0.1으로 복원해줘」**라고 요청하면:

1. `docs/releases/v0.1.md`를 기준 스펙으로 읽음  
2. `git checkout v0.1 -- .` 또는 `git reset --hard v0.1` (사용자 확인 후)  
3. `v0.1.md`의 **복원 후 확인 체크리스트** 실행  

> DB/Redis 데이터는 Git에 포함되지 않습니다. 스키마는 BE 기동 시 migration으로 재적용됩니다.
