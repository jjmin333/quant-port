import { describe, expect, it } from "vitest";
import { detectMobileFromUA, isCompactShell, resolveShellMode } from "@/lib/shell-mode";

const IPHONE =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1";
const ANDROID =
  "Mozilla/5.0 (Linux; Android 14; SM-S918N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Mobile Safari/537.36";
const ANDROID_TABLET =
  "Mozilla/5.0 (Linux; Android 13; SM-X700) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36";
const WINDOWS_CHROME =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36";
const MAC_SAFARI =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15";

describe("detectMobileFromUA", () => {
  it("아이폰·안드로이드 폰을 잡는다", () => {
    expect(detectMobileFromUA(IPHONE)).toBe(true);
    expect(detectMobileFromUA(ANDROID)).toBe(true);
  });

  it("안드로이드 태블릿도 좁은 화면으로 본다 — `Mobile`이 없어도 폭은 1024 미만이다", () => {
    expect(detectMobileFromUA(ANDROID_TABLET)).toBe(true);
  });

  it("데스크톱은 잡지 않는다", () => {
    expect(detectMobileFromUA(WINDOWS_CHROME)).toBe(false);
    // iPadOS 13+ 사파리가 이 UA를 보낸다 — 여기서는 못 가른다(마운트 뒤 보정).
    expect(detectMobileFromUA(MAC_SAFARI)).toBe(false);
  });

  it("UA가 비어 있으면 데스크톱으로 본다", () => {
    expect(detectMobileFromUA("")).toBe(false);
  });

  it("서버가 찍은 값이 그대로 compact 셸로 이어진다", () => {
    expect(isCompactShell(resolveShellMode(false, detectMobileFromUA(IPHONE)))).toBe(true);
    expect(isCompactShell(resolveShellMode(false, detectMobileFromUA(WINDOWS_CHROME)))).toBe(false);
  });
});
