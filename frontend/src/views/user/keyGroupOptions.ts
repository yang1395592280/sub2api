import type { Group, GroupPlatform, SubscriptionType } from "@/types";

export const OPENAI_AUTO_CHEAPEST_GROUP_VALUE = "openai_auto_cheapest" as const;

export type KeyGroupOptionValue = number | typeof OPENAI_AUTO_CHEAPEST_GROUP_VALUE;

export interface KeyGroupOption {
  value: KeyGroupOptionValue;
  label: string;
  description: string | null;
  rate: number | null;
  userRate: number | null;
  peakRateEnabled?: boolean;
  peakStart?: string;
  peakEnd?: string;
  peakRateMultiplier?: number;
  dynamicBillingEnabled?: boolean;
  dynamicBillingMin?: number;
  dynamicBillingMax?: number;
  subscriptionType: SubscriptionType;
  platform: GroupPlatform;
  kind?: "openai_auto_cheapest";
  [key: string]: unknown;
}

// 保持 /groups/available 的顺序；后端已按 sort_order asc, id asc 返回。
export function buildKeyGroupOptions(
  groups: Group[],
  userGroupRates: Record<number, number>,
  options: {
    includeOpenAIAutoCheapest?: boolean
    openAIAutoCheapestLabel?: string
    openAIAutoCheapestDescription?: string
    openAIDynamicProfitMarkup?: number
  } = {},
): KeyGroupOption[] {
  const profitMarkup = Math.max(0, Number(options.openAIDynamicProfitMarkup) || 0)
  const result: KeyGroupOption[] = groups.map((group) => ({
    value: group.id,
    label: group.name,
    description: group.description,
    rate: group.rate_multiplier,
    userRate: userGroupRates[group.id] ?? null,
    peakRateEnabled: group.peak_rate_enabled,
    peakStart: group.peak_start,
    peakEnd: group.peak_end,
    peakRateMultiplier: group.peak_rate_multiplier,
    dynamicBillingEnabled: group.dynamic_billing_enabled,
    dynamicBillingMin: group.dynamic_billing_enabled
      ? group.upstream_price_grouping_min + profitMarkup
      : undefined,
    dynamicBillingMax: group.dynamic_billing_enabled
      ? group.upstream_price_grouping_max + profitMarkup
      : undefined,
    subscriptionType: group.subscription_type,
    platform: group.platform,
  }));
  if (options.includeOpenAIAutoCheapest && groups.some((group) => group.platform === "openai")) {
    result.unshift({
      value: OPENAI_AUTO_CHEAPEST_GROUP_VALUE,
      label: options.openAIAutoCheapestLabel ?? "OpenAI 自动选择最优惠分组",
      description:
        options.openAIAutoCheapestDescription ??
        "按当前可用账号池自动使用最低倍率 OpenAI 分组",
      rate: null,
      userRate: null,
      subscriptionType: "standard",
      platform: "openai",
      kind: "openai_auto_cheapest",
    });
  }
  return result;
}
