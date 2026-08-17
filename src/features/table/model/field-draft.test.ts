import { expect, test } from "vitest";
import { newDraft, toBlocks, toDraft, withType } from "./field-draft";
import type { Field } from "./types";

const field = (label: string, slug: string) => ({ label, slug }) as Field;

test("новое поле без имени называется своим типом, повтор разводится числом", () => {
  const taken = [field("Status", "status")];

  expect(newDraft("STATUS", "", taken)).toMatchObject({ label: "Status 2", slug: "status_2" });
  expect(newDraft("STATUS", " Этап ", taken)).toMatchObject({ label: "Этап" });
});

test("STATUS заводится с вариантами: без них поле не выбирается вообще", () => {
  const draft = newDraft("STATUS", "", []);

  expect(draft.groups.todo).toHaveLength(1);
  expect(draft.groups.progress).toHaveLength(1);
  expect(draft.groups.complete).toHaveLength(1);
  // Обычному типу варианты не нужны.
  expect(newDraft("SINGLE_LINE", "", []).groups.todo).toHaveLength(0);
});

test("смена типа не теряет уже заведённые варианты", () => {
  const draft = newDraft("STATUS", "Этап", []);
  const renamed = { ...draft, groups: { ...draft.groups, todo: [{ label: "Новый", color: "red" as const }] } };

  expect(withType(renamed, "MULTISELECT").groups.todo).toEqual([{ label: "Новый", color: "red" }]);
});

test("переименование варианта не меняет его значения: под ним лежат строки", () => {
  const field = {
    type: "STATUS",
    label: "Этап",
    labels: {},
    slug: "etap",
    options: new Map([
      ["Progress_slug", { value: "Progress_slug", label: "Progress", labels: {}, color: null, icon: null, group: "progress" as const }],
    ]),
    attributes: {},
    raw: {},
  } as unknown as Field;

  expect(toDraft(field, "en").groups.progress).toEqual([
    { label: "Progress", color: "gray", value: "Progress_slug" },
  ]);
});

/*
 * Блоки списка типов. Дыра в полстроки возвращается молча — от одного
 * нового типа в группе, — поэтому правило проверяется, а не вычитывается.
 */
const group = (n: number) =>
  Array.from({ length: n }, (_, i) => ({ type: `t${i}`, label: `l${i}` }));

test("нечётная группа склеивается со следующей, чтобы блок кончился на границе строки", () => {
  // Текст (7) + связь (1) = 8: черта встаёт ровно под целой строкой.
  const blocks = toBlocks([group(7), group(1), group(4)]);

  expect(blocks.map((block) => block.length)).toEqual([8, 4]);
  expect(blocks.every((block) => block.length % 2 === 0)).toBe(true);
});

test("чётная группа остаётся своим блоком", () => {
  expect(toBlocks([group(4), group(2)]).map((b) => b.length)).toEqual([4, 2]);
});

test("склеивание идёт, пока сумма не станет чётной", () => {
  // 3 + 4 = 7 — всё ещё нечётно, берём и третью: 3 + 4 + 5 = 12.
  expect(toBlocks([group(3), group(4), group(5)]).map((b) => b.length)).toEqual([12]);
});

test("нечётный хвост остаётся блоком: полстроки внизу — это конец списка", () => {
  expect(toBlocks([group(2), group(3)]).map((b) => b.length)).toEqual([2, 3]);
  expect(toBlocks([])).toEqual([]);
});

test("порядок типов не меняется", () => {
  const a = [{ type: "a", label: "a" }];
  const b = [{ type: "b", label: "b" }];

  expect(toBlocks([a, b])[0]?.map((item) => item.type)).toEqual(["a", "b"]);
});
