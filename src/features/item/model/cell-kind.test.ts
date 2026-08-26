import { expect, test } from "vitest";
import type { Field } from "@/features/table";
import { editorKind } from "./cell-kind";

function field(patch: Partial<Field> = {}): Field {
  return {
    id: "f1",
    slug: "title",
    label: "Название",
    labels: {},
    type: "SINGLE_LINE",
    relationId: null,
    options: new Map(),
    multilanguage: false,
    required: false,
    validation: null,
    editable: true,
    locked: false,
    attributes: {},
    raw: {},
    ...patch,
  };
}

/*
 * Два запрета, и они разные. «Только чтение» ставит админ ПОЛЮ — это
 * про правку заведённой записи: «Номер договора» задают один раз
 * и больше не трогают. Право отнимает роль У ЧЕЛОВЕКА — это про него,
 * и заведение записи не должно становиться дырой в обход права.
 */
test("«только чтение» админа не действует при заведении записи", () => {
  const readOnly = field({ editable: false });

  expect(editorKind(readOnly)).toBe(null);
  expect(editorKind(readOnly, true)).toBe("text");
});

test("запрет роли действует и при заведении", () => {
  const denied = field({ editable: false, locked: true });

  expect(editorKind(denied)).toBe(null);
  expect(editorKind(denied, true)).toBe(null);
});

/* Вычисляемое поле не заполняет никто: значение считает бэкенд. */
test("вычисляемое поле закрыто в обоих случаях", () => {
  const generated = field({ type: "INCREMENT_ID" });

  expect(editorKind(generated)).toBe(null);
  expect(editorKind(generated, true)).toBe(null);
});

/* Выбор без вариантов — не выбор: редактор открылся бы пустым списком. */
test("список без вариантов редактора не даёт даже при заведении", () => {
  const empty = field({ type: "MULTISELECT" });

  expect(editorKind(empty, true)).toBe(null);
});
