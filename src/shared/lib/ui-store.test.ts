import { describe, expect, it } from "vitest";
import {
  clampDrawerWidth,
  clampSidebarWidth,
  DRAWER_MAX_WIDTH,
  DRAWER_MIN_WIDTH,
  SIDEBAR_MAX_WIDTH,
  SIDEBAR_MIN_WIDTH,
  useUi,
} from "./ui-store";

describe("clampSidebarWidth", () => {
  it("держит ширину в границах и отдаёт целое число", () => {
    expect(clampSidebarWidth(0)).toBe(SIDEBAR_MIN_WIDTH);
    expect(clampSidebarWidth(9999)).toBe(SIDEBAR_MAX_WIDTH);
    expect(clampSidebarWidth(300.4)).toBe(300);
  });
});

describe("clampDrawerWidth", () => {
  it("держит ширину панели в своих границах", () => {
    expect(clampDrawerWidth(10)).toBe(DRAWER_MIN_WIDTH);
    expect(clampDrawerWidth(9999)).toBe(DRAWER_MAX_WIDTH);
    expect(clampDrawerWidth(700.6)).toBe(701);
  });
});

describe("toggleMenu", () => {
  it("раскрывает и схлопывает папку, не трогая соседние", () => {
    const { toggleMenu } = useUi.getState();

    toggleMenu("orders");
    toggleMenu("clients");
    expect(useUi.getState().expandedMenus).toEqual(["orders", "clients"]);

    toggleMenu("orders");
    expect(useUi.getState().expandedMenus).toEqual(["clients"]);
  });
});
