import { expect, test } from "vitest";
import { EMPTY_MENU_FORM, menuAttributes, type MenuFormValue } from "./MenuFormDialog";

const value = (over: Partial<MenuFormValue> = {}): MenuFormValue => ({
  ...EMPTY_MENU_FORM,
  ...over,
});

test("ссылка наружу и встроенная страница — разные ключи", () => {
  // Так их различала и старая админка: website_link открывался рамкой
  // внутри админки, link — новой вкладкой.
  expect(menuAttributes("LINK", value({ href: "https://x.dev", embed: false }))).toEqual({
    link: "https://x.dev",
  });
  expect(menuAttributes("LINK", value({ href: "https://x.dev", embed: true }))).toEqual({
    website_link: "https://x.dev",
  });
});

test("у хранилища уезжает папка, у остальных типов — ничего", () => {
  expect(menuAttributes("MINIO_FOLDER", value({ folder: "media" }))).toEqual({ path: "media" });
  expect(menuAttributes("FOLDER", value({ href: "https://x.dev", folder: "media" }))).toEqual({});
  expect(menuAttributes("TABLE", value({ folder: "media" }))).toEqual({});
});

test("параметры микрофронтенда: пары без ключа не сохраняются", () => {
  // Пустая строка остаётся в форме от нажатой и незаполненной кнопки
  // «Добавить параметр»; ремоуту она не нужна.
  const params = [
    { key: "mode", value: "compact" },
    { key: "  ", value: "потерянное" },
    { key: "tab", value: "" },
  ];

  expect(menuAttributes("MICROFRONTEND", value({ params }))).toEqual({
    params: [
      { key: "mode", value: "compact" },
      // Пустое ЗНАЧЕНИЕ законно: ключ без значения ремоут читает как флаг.
      { key: "tab", value: "" },
    ],
  });
});
