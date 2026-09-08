import { expect, test } from "vitest";
import { DEFAULT_SESSION_LIMIT, toClientType } from "./client-types";

/*
 * Предел сессий приехал нулём — значит колонки в ответе не было
 * (список на mongo-проектах отдаётся урезанным). Ноль означал бы
 * «ни одной сессии»: показав его, форма записала бы его обратно
 * и заперла аудиторию снаружи. В базе у колонки значение
 * по умолчанию 50 — его и показываем.
 */
test("нулевой предел сессий читается как «не приехал», а не как ноль", () => {
  expect(toClientType({ guid: "c1", session_limit: 0 }).sessionLimit).toBe(
    DEFAULT_SESSION_LIMIT,
  );
  expect(toClientType({ guid: "c1", session_limit: 3 }).sessionLimit).toBe(3);
});

/*
 * Стратегию подтверждения мы не показываем, но обязаны вернуть её
 * бэкенду при правке нетронутой: он пишет её в базу всегда. Пустое
 * значение читается как UNDECIDED — то же, что стоит в базе по
 * умолчанию, а не как «не трогать».
 */
test("стратегия подтверждения не теряется", () => {
  expect(toClientType({ guid: "c1", confirm_by: "PHONE" }).confirmBy).toBe("PHONE");
  expect(toClientType({ guid: "c1" }).confirmBy).toBe("UNDECIDED");
});

test("имя без подписи не превращается в пустую строку", () => {
  expect(toClientType({ guid: "c1", name: "  Клиенты " }).name).toBe("Клиенты");
  expect(toClientType({ guid: "c1", name: "" }).name).toBe("c1");
});
