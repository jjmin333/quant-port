<#
.SYNOPSIS
    종목 로고 월간 갱신 — 국내(네이버) + 해외(FMP).

.DESCRIPTION
    로고는 **런타임 데이터가 아니라 FE 이미지에 구워지는 정적 자산**이다
    (`fe/public/logos`). 컨테이너에 마운트된 볼륨이 아니므로 워커가 밤에 갱신할 수
    없고, 갱신하려면 이 스크립트를 돌린 뒤 FE를 다시 빌드해 배포해야 한다.

    한 달에 한 번이면 충분하다. 로고가 바뀌는 사건은 상장·사명 변경·CI 교체뿐이고
    셋 다 드물다. 그래서 이 스크립트는 **델타만** 받는다(`-delta-only`) — 이미 파일이
    있는 종목은 건너뛰므로, 두 번째 실행부터는 새로 상장된 것과 지난번에 실패한 것만
    다시 시도한다.

    -StoreUS를 주면 해외 로고도 내려받아 `fe/public/logos/us/*.png`로 저장한다.
    기본값이 아닌 이유는 용량이다: 실측 종목당 ~7.8KB, 전체 약 11,200종목이면
    **약 87MB**가 저장소에 더해진다(국내는 4,247개 35MB). 저장하지 않으면 화면이
    지금처럼 FMP CDN URL을 런타임에 부른다.

.EXAMPLE
    .\scripts\refresh-logos.ps1
    국내만 델타 갱신 (해외는 FMP 확인만).

.EXAMPLE
    .\scripts\refresh-logos.ps1 -StoreUS
    해외 로고까지 내려받아 저장.
#>
param(
    [switch]$StoreUS,
    # 해외 로고를 건드리지 않고 국내만 돌린다 — 국내는 네이버 CDN이라 훨씬 빠르다.
    [switch]$DomesticOnly
)

$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

$outDir = '../fe/public/logos'
$args = @('-from-db', '-seed=false', '-delta-only', "-out=$outDir")

if ($DomesticOnly) {
    # `-us-max=0`은 「전부」라서 쓸 수 없다. 해외를 빼려면 목록에서 빼야 한다.
    Write-Host '국내 로고만 갱신합니다.' -ForegroundColor Cyan
    $args += '-us-max=1'
} elseif ($StoreUS) {
    Write-Host '국내 + 해외 로고를 갱신하고, 해외는 파일로 저장합니다 (~7.8KB/종목).' -ForegroundColor Cyan
    $args += '-store-us'
} else {
    Write-Host '국내 로고를 갱신하고, 해외는 FMP에 그림이 있는지만 확인합니다.' -ForegroundColor Cyan
}

go run ./cmd/migrate-logos @args
if ($LASTEXITCODE -ne 0) { throw "migrate-logos failed ($LASTEXITCODE)" }

Write-Host ''
Write-Host '완료. 로고는 FE 이미지에 구워지므로 배포해야 화면에 반영됩니다:' -ForegroundColor Yellow
Write-Host '  cd ../fe; .\scripts\deploy.ps1' -ForegroundColor Yellow
