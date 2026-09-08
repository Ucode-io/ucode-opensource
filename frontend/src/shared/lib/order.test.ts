import { expect, test } from "vitest";
import { moveBefore } from "./order";

test("перетащенный встаёт перед целью — и сверху вниз, и снизу вверх", () => {
  expect(moveBefore(["a", "b", "c", "d"], "a", "c")).toEqual(["b", "a", "c", "d"]);
  expect(moveBefore(["a", "b", "c", "d"], "d", "b")).toEqual(["a", "d", "b", "c"]);
});

test("бросок на себя и мимо списка порядок не меняют", () => {
  expect(moveBefore(["a", "b"], "a", "a")).toEqual(["a", "b"]);
  expect(moveBefore(["a", "b"], "a", "z")).toEqual(["a", "b"]);
});
