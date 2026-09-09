import { expect, test } from "vitest";
import { applyEvent, emptyRun, hasWork, summaryCount, type StreamEvent } from "./run";

/*
 * Свёртка потока — единственное место, где «что происходит на сервере»
 * превращается в то, что видно. Ошибка здесь не падает, а тихо рисует
 * неправду: таблица дважды, вечный крутящийся кружок, пустая карточка
 * шагов под обычной фразой.
 */

const fold = (events: StreamEvent[]) => events.reduce(applyEvent, emptyRun());

test("«готово» дополняет свою строку, а не добавляет вторую", () => {
  const run = fold([
    { type: "table_start", message: "Создаю таблицу", data: { action: "table", status: "started", table: "product" } },
    { type: "table_done", message: "Таблица готова", value: "product", data: { action: "table", status: "done", table: "product" } },
  ]);

  expect(run.steps).toHaveLength(1);
  expect(run.steps[0]).toMatchObject({ status: "done", message: "Таблица готова", value: "product" });
});

test("«готово» находит СВОЮ таблицу, а не последнюю начатую", () => {
  const run = fold([
    { type: "table_start", message: "Создаю", data: { action: "table", status: "started", table: "product" } },
    { type: "table_start", message: "Создаю", data: { action: "table", status: "started", table: "order" } },
    { type: "table_done", message: "Готово", data: { action: "table", status: "done", table: "product" } },
  ]);

  expect(run.steps).toHaveLength(2);
  expect(run.steps[0]).toMatchObject({ table: "product", status: "done" });
  expect(run.steps[1]).toMatchObject({ table: "order", status: "started" });
});

test("«готово» без пары становится отдельной строкой", () => {
  // Так приходит пропущенная таблица: она уже была, «начал» не было.
  const run = fold([
    { type: "table_done", message: "Таблица уже есть", data: { action: "table", status: "skipped", table: "product" } },
  ]);

  expect(run.steps).toHaveLength(1);
  expect(run.steps[0]).toMatchObject({ status: "skipped" });
});

test("событие без data — мысль вслух, а не действие", () => {
  const run = fold([{ type: "progress", icon: "brain", message: "Смотрю схему проекта" }]);

  expect(run.steps[0]).toMatchObject({ kind: "reasoning", icon: "brain" });
});

test("предупреждение не обрывает работу, ошибка обрывает", () => {
  const warned = fold([{ type: "warning", message: "Не удалось создать поле", data: { action: "field" } }]);
  expect(warned.active).toBe(true);
  expect(warned.steps[0]).toMatchObject({ kind: "step", status: "failed" });

  const failed = fold([{ type: "error", message: "Сервис недоступен" }]);
  expect(failed.active).toBe(false);
  expect(failed.error).toBe("Сервис недоступен");
});

test("последнее событие приносит текст ответа и итог", () => {
  const run = fold([
    { type: "provider", data: { provider: "gemini", coder_model: "2.5-pro" } },
    {
      type: "done",
      message: "хвост",
      data: {
        message: { content: "Готово: таблица product создана." },
        summary: { tables: 1, fields: 3 },
        duration_sec: 12,
      },
    },
  ]);

  expect(run.active).toBe(false);
  expect(run.provider).toBe("gemini · 2.5-pro");
  expect(run.content).toBe("Готово: таблица product создана.");
  expect(run.duration).toBe(12);
  expect(summaryCount(run.summary)).toBe(4);
});

test("разговор без работы не показывает карточку шагов", () => {
  // Провайдер, мысль вслух и пустой итог — так выглядит простой ответ.
  const talk = fold([
    { type: "provider", data: { provider: "gemini" } },
    { type: "progress", icon: "brain", message: "Отвечаю" },
    { type: "done", data: { message: { content: "Да, поле уже есть." }, summary: {} } },
  ]);

  expect(hasWork(talk)).toBe(false);

  const work = fold([
    { type: "table_done", message: "Готово", data: { action: "table", status: "done", table: "product" } },
    { type: "done", data: { message: { content: "Создал." }, summary: { tables: 1 } } },
  ]);

  expect(hasWork(work)).toBe(true);
});
