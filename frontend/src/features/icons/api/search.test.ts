import { expect, test } from "vitest";
import { isOutline } from "./search";

test("заливные варианты Tabler не попадают в выбор", () => {
  // Иначе пикер предлагал бы и контурную, и заливную версию одного значка,
  // а смысл ограничения коллекцией — единый стиль.
  expect(isOutline("tabler:home")).toBe(true);
  expect(isOutline("tabler:home-filled")).toBe(false);
  expect(isOutline("tabler:circle-filled")).toBe(false);
  expect(isOutline("tabler:filled-something")).toBe(true);
});
