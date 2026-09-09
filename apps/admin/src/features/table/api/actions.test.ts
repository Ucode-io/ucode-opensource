import { expect, test } from "vitest";
import { isMicrofrontend, toAction } from "./actions";

test("право роли на действие: запрет строгий, разрешение по умолчанию", () => {
  // Запись прав есть и в ней запрет — не показываем.
  expect(toAction({ id: "a", action_permission: { id: "p", permission: false } }).allowed).toBe(
    false,
  );
  expect(toAction({ id: "a", action_permission: { id: "p", permission: true } }).allowed).toBe(true);

  /*
   * Записи прав нет: `permission` приходит тем же false, но это
   * COALESCE, а не запрет. Так выглядит роль, заведённая уже после
   * действия, — записи ей никто не создаёт, и запретом это считать
   * нельзя: она потеряла бы все действия таблицы навсегда.
   */
  expect(toAction({ id: "a", action_permission: { id: "", permission: false } }).allowed).toBe(true);
  expect(toAction({ id: "a" }).allowed).toBe(true);
});

test("тип функции решает, звать действие или показывать", () => {
  // Список отдаёт функцию массивом из-за GROUP BY, но элемент один.
  expect(isMicrofrontend(toAction({ id: "a", functions: [{ type: "MICRO_FRONTEND" }] }))).toBe(true);
  expect(isMicrofrontend(toAction({ id: "a", functions: [{ type: "FUNCTION" }] }))).toBe(false);

  // Типа нет — обычное действие: показывать нечего, звать есть что.
  expect(isMicrofrontend(toAction({ id: "a" }))).toBe(false);
});
