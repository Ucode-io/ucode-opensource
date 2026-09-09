import { expect, test, vi } from "vitest";

vi.stubEnv("VITE_ICON_CDN_URL", "https://cdn.u-code.io/icons/");

const { resolveIconSource } = await import("./dynamic-icon");

test("имя с двоеточием — это Iconify", () => {
  expect(resolveIconSource("mdi:home")).toEqual({
    kind: "inline",
    url: "https://api.iconify.design/mdi/home.svg",
  });
});

test("имя файла берётся из нашего CDN", () => {
  expect(resolveIconSource("folder-new.svg")).toEqual({
    kind: "inline",
    url: "https://cdn.u-code.io/icons/folder-new.svg",
  });
});

test("чужой URL вставляется картинкой, а не инлайном", () => {
  // Инлайнить чужую разметку внутрь своей страницы нельзя.
  expect(resolveIconSource("https://example.com/logo.svg")).toEqual({
    kind: "img",
    url: "https://example.com/logo.svg",
  });
});

test("пустое и битое имя не дают источника — покажется своя иконка", () => {
  expect(resolveIconSource("")).toBeNull();
  expect(resolveIconSource("mdi:")).toBeNull();
  expect(resolveIconSource(":home")).toBeNull();
});
