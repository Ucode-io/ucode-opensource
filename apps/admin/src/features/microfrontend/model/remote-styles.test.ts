import { expect, test, vi } from "vitest";
import { layerRemoteStyles, scopeSelector } from "./remote-styles";

/*
 * DOM подделан, а не поднят: в проекте нет ни jsdom, ни happy-dom, и
 * заводить их ради трёх строк незачем. Функция знает о документе ровно
 * `querySelectorAll`, `createElement` и `head` — этого и хватает.
 */
const ENTRY = "https://app.example.com/assets/remoteEntry.js";
const THEIRS = "https://app.example.com/assets/style.abc123.css";

type FakeStyle = { dataset: Record<string, string>; textContent: string; remove(): void };

function withStylesheets(...hrefs: string[]) {
  const links = hrefs.map((href) => ({ href, media: "" }));
  const head: FakeStyle[] = [];

  vi.stubGlobal("MutationObserver", class { observe() {} disconnect() {} });
  vi.stubGlobal("document", {
    querySelectorAll: () => links,
    createElement: (): FakeStyle => {
      const style: FakeStyle = {
        dataset: {},
        textContent: "",
        remove: () => void head.splice(head.indexOf(style), 1),
      };
      return style;
    },
    head: { append: (style: FakeStyle) => void head.push(style) },
  });
  return { links, head };
}

test("стили ремоута уезжают в слой `remote` — свои не трогаем", () => {
  const { links, head } = withStylesheets(THEIRS, "http://localhost:7777/assets/index.css");
  const [theirs, ours] = links;

  layerRemoteStyles(ENTRY);

  // Оригинал гасим: вне слоёв он бил бы наши утилиты.
  expect(theirs?.media).toBe("not all");
  expect(head.map((style) => style.textContent)).toEqual([
    `@import url("${THEIRS}") layer(remote);`,
  ]);
  // Свои не трогаем никогда: у них другой origin.
  expect(ours?.media).toBe("");
});

test("уход гасит, возврат включает заново", () => {
  const { head } = withStylesheets(THEIRS);

  layerRemoteStyles(ENTRY)();
  expect(head).toEqual([]);

  /*
   * Копию делаем снова мы: оригинал остаётся погашенным в `<head>`,
   * а плагин федерации второй раз ссылку не создаст (`seen` в его
   * `dynamicLoadingCss`).
   */
  layerRemoteStyles(ENTRY);
  expect(head).toHaveLength(1);
});

test("битый адрес сборки ничего не трогает", () => {
  const { links, head } = withStylesheets("http://localhost:7777/assets/index.css");

  layerRemoteStyles("")();

  expect(links[0]?.media).toBe("");
  expect(head).toEqual([]);
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
