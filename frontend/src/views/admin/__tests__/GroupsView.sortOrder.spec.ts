import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, h, nextTick } from "vue";
import { flushPromises, mount } from "@vue/test-utils";

import GroupsView from "../GroupsView.vue";
import type { AdminGroup } from "@/types";

const {
  listGroups,
  getAllGroups,
  updateSortOrder,
  getModelsListCandidates,
  getUsageSummary,
  getCapacitySummary,
  getLiveCapability,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  listGroups: vi.fn(),
  getAllGroups: vi.fn(),
  updateSortOrder: vi.fn(),
  getModelsListCandidates: vi.fn(),
  getUsageSummary: vi.fn(),
  getCapacitySummary: vi.fn(),
  getLiveCapability: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}));

vi.mock("@/api/admin", () => ({
  adminAPI: {
    groups: {
      list: listGroups,
      getAll: getAllGroups,
      updateSortOrder,
      getModelsListCandidates,
      getUsageSummary,
      getCapacitySummary,
      getLiveCapability,
    },
  },
}));

vi.mock("@/stores/app", () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}));

vi.mock("@/stores/onboarding", () => ({
  useOnboardingStore: () => ({
    shouldShowTour: false,
    startTour: vi.fn(),
  }),
}));

vi.mock("vue-i18n", async () => {
  const actual = await vi.importActual<typeof import("vue-i18n")>("vue-i18n");
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  };
});

vi.mock("vue-draggable-plus", () => ({
  VueDraggable: defineComponent({
    props: {
      modelValue: {
        type: Array,
        default: () => [],
      },
    },
    setup(_props, { slots }) {
      return () => h("div", { class: "draggable-stub" }, slots.default?.());
    },
  }),
}));

const AppLayoutStub = { template: "<div><slot /></div>" };
const TablePageLayoutStub = {
  template:
    '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
};
const DataTableStub = { template: "<div />" };
const PaginationStub = { template: "<div />" };
const BaseDialogStub = defineComponent({
  props: {
    show: {
      type: Boolean,
      default: false,
    },
  },
  setup(props, { slots }) {
    return () =>
      props.show
        ? h("section", { "data-testid": "dialog" }, [
            slots.default?.(),
            slots.footer?.(),
          ])
        : null;
  },
});
const EmptyStub = { template: "<div />" };
const SelectStub = defineComponent({
  inheritAttrs: false,
  setup() {
    return () => h("div");
  },
});
const IconStub = defineComponent({
  props: {
    name: {
      type: String,
      required: true,
    },
  },
  setup(props) {
    return () => h("span", { "data-icon": props.name });
  },
});

function group(partial: Partial<AdminGroup>): AdminGroup {
  return {
    id: 0,
    name: "",
    description: "",
    platform: "anthropic",
    status: "active",
    rate_multiplier: 1,
    is_exclusive: false,
    subscription_type: "standard",
    sort_order: 0,
    account_count: 0,
    active_account_count: 0,
    rate_limited_account_count: 0,
    ...partial,
  } as AdminGroup;
}

function mountGroupsView() {
  return mount(GroupsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: PaginationStub,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: EmptyStub,
        EmptyState: EmptyStub,
        Select: SelectStub,
        PlatformIcon: EmptyStub,
        Icon: IconStub,
        GroupMembersModal: EmptyStub,
        GroupRateMultipliersModal: EmptyStub,
        GroupRPMOverridesModal: EmptyStub,
        GroupCapacityBadge: EmptyStub,
      },
    },
  });
}

describe("GroupsView sort order modal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listGroups.mockResolvedValue({ items: [], total: 0, pages: 0 });
    getAllGroups.mockResolvedValue([]);
    updateSortOrder.mockResolvedValue({ message: "ok" });
    getModelsListCandidates.mockResolvedValue([]);
    getUsageSummary.mockResolvedValue([]);
    getCapacitySummary.mockResolvedValue([]);
    getLiveCapability.mockResolvedValue({ supported: true });
  });

  it("opens groups sorted by sort_order and saves arrow-adjusted order", async () => {
    getAllGroups.mockResolvedValue([
      group({ id: 1, name: "Middle", sort_order: 20 }),
      group({ id: 2, name: "First", sort_order: 10 }),
      group({ id: 3, name: "Last", sort_order: 30 }),
    ]);

    const wrapper = mountGroupsView();
    await flushPromises();

    await wrapper.find("[data-testid='open-sort-modal']").trigger("click");
    await flushPromises();

    expect(wrapper.text()).toMatch(/First[\s\S]*Middle[\s\S]*Last/);

    await wrapper.find("[data-testid='move-sort-1-up']").trigger("click");
    await nextTick();
    await wrapper.find("[data-testid='save-sort-order']").trigger("click");
    await flushPromises();

    expect(updateSortOrder).toHaveBeenCalledWith([
      { id: 1, sort_order: 0 },
      { id: 2, sort_order: 10 },
      { id: 3, sort_order: 20 },
    ]);
    expect(showSuccess).toHaveBeenCalledWith("admin.groups.sortOrderUpdated");
  });
});
