import { describe, expect, it, vi } from "vitest";

import { getSettings } from "@/api/admin/settings";
import { apiClient } from "@/api/client";

vi.mock("@/api/client", () => ({
  apiClient: {
    get: vi.fn(),
  },
}));

describe("admin settings cache policy", () => {
  it("adds a cache-busting timestamp when loading settings", async () => {
    vi.mocked(apiClient.get).mockResolvedValueOnce({ data: { site_name: "Loomex" } });

    const settings = await getSettings();

    expect(apiClient.get).toHaveBeenCalledWith("/admin/settings", {
      params: { _ts: expect.any(Number) },
    });
    expect(settings.site_name).toBe("Loomex");
  });
});
