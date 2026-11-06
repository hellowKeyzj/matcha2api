import { describe, expect, it } from "vitest";

import {
  normalizeGroupOpenAIFast,
  supportsGroupOpenAIFast,
} from "../groupsOpenAIFast";
import en from "@/i18n/locales/en/admin/overview";
import zh from "@/i18n/locales/zh/admin/overview";

describe("groupsOpenAIFast", () => {
  it("supports OpenAI and composite groups", () => {
    expect(supportsGroupOpenAIFast("openai")).toBe(true);
    expect(supportsGroupOpenAIFast("composite")).toBe(true);
    expect(supportsGroupOpenAIFast("anthropic")).toBe(false);
  });

  it("clears stale enabled state on unsupported platforms", () => {
    expect(normalizeGroupOpenAIFast("openai", true)).toBe(true);
    expect(normalizeGroupOpenAIFast("composite", true)).toBe(true);
    expect(normalizeGroupOpenAIFast("anthropic", true)).toBe(false);
    expect(normalizeGroupOpenAIFast("openai", false)).toBe(false);
  });

  it("explains Ultra Fast entitlements, fallback and Free Fast in both locales", () => {
    const zhFast = zh.groups.openaiFast;
    expect(zhFast.forceUltrafast).toContain("ultrafast");
    expect(zhFast.ultrafastHint).toContain("GPT-6.2");
    expect(zhFast.ultrafastHint).toContain("Pro 200 支持 Sol");
    expect(zhFast.ultrafastHint).toContain("Pro 500 支持 Sol 和 Astra");
    expect(zhFast.ultrafastHint).toContain("其他账号使用其目录明确提供的档位");
    expect(zhFast.ultrafastHint).toContain("不可用时，若同时开启强制 Fast 则使用 Fast");
    expect(zhFast.ultrafastHint).toContain("否则跟随客户端");
    expect(zhFast.ultrafastHint).toContain("免费 Fast 不包含 Ultra Fast");
    expect(zhFast.ultrafastHint).toContain("GPT-6 Astra");
    expect(zhFast.ultrafastHint).toContain("GPT-6.1 Sol");
    expect(zhFast.freeHint).toContain("priority 档位");
    expect(zhFast.freeHint).toContain("Standard 价格");

    const enFast = en.groups.openaiFast;
    expect(enFast.forceUltrafast).toContain("ultrafast");
    expect(enFast.ultrafastHint).toContain("Pro 200 includes GPT-6.2 Sol Ultrafast");
    expect(enFast.ultrafastHint).toContain("Pro 500 includes both Sol and Astra Ultrafast");
    expect(enFast.ultrafastHint).toContain("Other subscriptions use the tiers advertised by their account");
    expect(enFast.ultrafastHint).toContain("If unavailable, Force Fast takes precedence over the client tier");
    expect(enFast.ultrafastHint).toContain("GPT-6.2 Sol and Astra");
    expect(enFast.ultrafastHint).toContain("Free Fast excludes Ultra Fast");
    expect(enFast.ultrafastHint).toContain("GPT-6 Astra");
    expect(enFast.ultrafastHint).toContain("GPT-6.1 Sol");
    expect(enFast.freeHint).toContain("priority tier");
    expect(enFast.freeHint).toContain("equivalent Standard price");
  });

  it("provides localized switch labels", () => {
    expect(zh.groups.openaiFast).toMatchObject({
      title: expect.any(String),
      force: expect.any(String),
      hint: expect.stringContaining("service_tier=priority"),
      free: expect.any(String),
      freeHint: expect.stringContaining("Standard"),
    });
    expect(en.groups.openaiFast).toMatchObject({
      title: expect.any(String),
      force: expect.any(String),
      hint: expect.stringContaining("service_tier=priority"),
      free: expect.any(String),
      freeHint: expect.stringContaining("Standard"),
    });
  });
});
