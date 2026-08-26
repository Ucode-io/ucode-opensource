import { expect, test, vi } from "vitest";
import { scopeSelector, toggleRemoteStyles } from "./remote-styles";

/*
 * DOM подделан, а не поднят: в проекте нет ни jsdom, ни happy-dom, и
 * заводить их ради трёх строк незачем. Функция знает о документе ровно
 * `querySelectorAll` и два поля ссылки — этого и хватает.
 */
const ENTRY = "https://app.example.com/assets/remoteEntry.js";

function withStylesheets(...hrefs: string[]) {
  const links = hrefs.map((href) => ({ href, disabled: false }));
  vi.stubGlobal("document", { querySelectorAll: () => links });
  return links;
}

test("уходя с экрана, гасим стили ремоута — но не свои", () => {
  const [theirs, ours] = withStylesheets(
    "https://app.example.com/assets/style.abc123.css",
    "http://localhost:7777/assets/index.css",
  );

  toggleRemoteStyles(ENTRY, false);

  expect(theirs?.disabled).toBe(true);
  // Свои не трогаем никогда: у них другой origin.
  expect(ours?.disabled).toBe(false);
});

test("возвращаясь, включаем обратно", () => {
  const [theirs] = withStylesheets("https://app.example.com/assets/style.abc123.css");

  toggleRemoteStyles(ENTRY, false);
  toggleRemoteStyles(ENTRY, true);

  /*
   * Именно включаем, а не вставляем заново: узел остаётся в `<head>`,
   * потому что плагин федерации помнит вставленные адреса и второй раз
   * ссылку не создаст (`seen` в его `dynamicLoadingCss`).
   */
  expect(theirs?.disabled).toBe(false);
});

test("битый адрес сборки ничего не гасит", () => {
  const [ours] = withStylesheets("http://localhost:7777/assets/index.css");

  toggleRemoteStyles("", false);

  expect(ours?.disabled).toBe(false);
});

/*
 * Сужение — это не только «добавить префикс». Вес селектора обязан
 * остаться прежним, иначе чужой сброс поднимется над собственными
 * классами ремоута, и поедет уже он. Отсюда `:where()` вместо `#id`.
 */

test("сброс сужается до поддерева, не набирая веса", () => {
  expect(scopeSelector("button")).toBe(":where(#ucode-remote) button");
  expect(scopeSelector(":where(*)")).toBe(":where(#ucode-remote) :where(*)");
  expect(scopeSelector("*, ::before, ::after")).toBe(
    ":where(#ucode-remote) *, :where(#ucode-remote) ::before, :where(#ucode-remote) ::after",
  );
});

test("правила о корне документа переносятся НА узел ремоута", () => {
  // Иначе `body { font-family }` перекрасил бы всю админку.
  expect(scopeSelector("body")).toBe(":where(#ucode-remote)");
  expect(scopeSelector(":host, :root, [data-theme]")).toBe(
    ":where(#ucode-remote), :where(#ucode-remote), :where(#ucode-remote) [data-theme]",
  );
  // Класс на body остаётся предком узла — правило продолжает работать.
  expect(scopeSelector(".chakra-ui-light :root:not([data-theme])")).toBe(
    ".chakra-ui-light :where(#ucode-remote):not([data-theme])",
  );
});

test("повторный проход ничего не портит", () => {
  // Наблюдатель зовёт сужение на каждое изменение head.
  const once = scopeSelector("button");
  expect(scopeSelector(once)).toBe(once);
});
