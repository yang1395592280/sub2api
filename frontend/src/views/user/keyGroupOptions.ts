import type { Group, GroupPlatform, SubscriptionType } from "@/types";

export interface KeyGroupOption {
  value: number;
  label: string;
  description: string | null;
  rate: number;
  userRate: number | null;
  subscriptionType: SubscriptionType;
  platform: GroupPlatform;
  [key: string]: unknown;
}

// 保持 /groups/available 的顺序；后端已按 sort_order asc, id asc 返回。
export function buildKeyGroupOptions(
  groups: Group[],
  userGroupRates: Record<number, number>,
): KeyGroupOption[] {
  return groups.map((group) => ({
    value: group.id,
    label: group.name,
    description: group.description,
    rate: group.rate_multiplier,
    userRate: userGroupRates[group.id] ?? null,
    subscriptionType: group.subscription_type,
    platform: group.platform,
  }));
}
