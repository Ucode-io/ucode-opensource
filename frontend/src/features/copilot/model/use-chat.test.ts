import { expect, test } from "vitest";
import { reducer } from "./use-chat";
import { emptyRun, type Run } from "./run";

/*
 * Проверяется то, что видно на экране после остановки: незакрытый шаг
 * не должен остаться с вечно крутящимся кружком, а сама реплика —
 * пропасть вместе с уже сделанной работой.
 */

const asking = () => reducer({ chatId: "c", title: "", messages: [], run: null }, {
  type: "ask",
  content: "Создай таблицу",
});

/** Состояние посреди потока: одна таблица готова, вторая ещё делается. */
function midRun() {
  const state = asking();
  const run: Run = {
    ...emptyRun(),
    steps: [
      { id: "s1", kind: "step", icon: "database", message: "Таблица готова", status: "done" },
      { id: "s2", kind: "step", icon: "database", message: "Создаю таблицу", status: "started" },
    ],
  };
  return { ...state, run };
}

test("остановка гасит «идёт», но оставляет сделанное", () => {
  const state = reducer(midRun(), { type: "finish", stopped: true });

  expect(state.run).toBeNull();
  expect(state.messages).toHaveLength(2);

  const answer = state.messages[1]!;
  expect(answer.role).toBe("assistant");
  expect(answer.run?.stopped).toBe(true);
  expect(answer.run?.active).toBe(false);
  // Готовый шаг не тронут, незакрытый остался без состояния.
  expect(answer.run?.steps[0]?.status).toBe("done");
  expect(answer.run?.steps[1]?.status).toBeUndefined();
  expect(answer.run?.steps[1]?.message).toBe("Создаю таблицу");
});

test("обычное завершение статусы не трогает", () => {
  const state = reducer(midRun(), { type: "finish" });

  expect(state.messages[1]?.run?.stopped).toBe(false);
  expect(state.messages[1]?.run?.steps[1]?.status).toBe("started");
});

test("первая реплика становится именем беседы, вторая — нет", () => {
  const first = asking();
  expect(first.title).toBe("Создай таблицу");
  expect(first.run).not.toBeNull();

  const second = reducer(first, { type: "ask", content: "И поле цены" });
  expect(second.title).toBe("Создай таблицу");
  expect(second.messages).toHaveLength(2);
});

test("завершать нечего — состояние не меняется", () => {
  const idle = { chatId: "c", title: "", messages: [], run: null };

  expect(reducer(idle, { type: "finish" })).toBe(idle);
});
