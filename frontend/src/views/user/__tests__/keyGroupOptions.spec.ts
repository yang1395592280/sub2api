import { describe, expect, it } from "vitest";

import { buildKeyGroupOptions } from "../keyGroupOptions";
import type { Group } from "@/types";

function group(partial: Partial<Group>): Group {
  return {
    id: 0,
    name: "",
    description: null,
    platform: "anthropic",
    rate_multiplier: 1,
    is_exclusive: false,
    subscription_type: "standard",
    sort_order: 0,
    ...partial,
  } as Group;
}

describe("buildKeyGroupOptions", () => {
  it("preserves the available-groups API order for key creation", () => {
    const groups = [
      group({ id: 20, name: "Second", sort_order: 20 }),
      group({ id: 10, name: "First", sort_order: 10 }),
      group({ id: 30, name: "Third", sort_order: 30 }),
    ];

    const options = buildKeyGroupOptions(groups, { 10: 0.8, 30: 1.2 });

    expect(options.map((option) => option.value)).toEqual([20, 10, 30]);
    expect(options[1]).toMatchObject({
      value: 10,
      label: "First",
      userRate: 0.8,
    });
  });
});
