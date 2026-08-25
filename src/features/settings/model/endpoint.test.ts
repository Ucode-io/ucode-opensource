import { expect, test } from "vitest";
import { ENDPOINT_PREFIX, joinPath, placeholders, splitPath, unmatchedParams } from "./endpoint";

test("подстановки — только сегменты целиком в фигурных скобках", () => {
  expect(placeholders("/v1/object/{slug}/{id}")).toEqual(["slug", "id"]);
  expect(placeholders("/v1/object/order")).toEqual([]);
  // Пустые скобки — не имя, подставлять нечего.
  expect(placeholders("/v1/{}/x")).toEqual([]);
});

/*
 * Имя, которого нет во `from`, шлюз не заменяет ничем: путь уезжает
 * с фигурными скобками и отвечает 404. Это и ловим до сохранения.
 */
test("подстановка в «куда» без пары в «откуда» — ошибка правила", () => {
  expect(unmatchedParams("/x-api/order/{id}", "/v1/object/order/{id}")).toEqual([]);
  expect(unmatchedParams("/x-api/order", "/v1/object/{slug}")).toEqual(["slug"]);
});

test("приставка пути постоянна и не удваивается", () => {
  expect(joinPath(ENDPOINT_PREFIX, "order/{id}")).toBe("/x-api/order/{id}");
  expect(joinPath(ENDPOINT_PREFIX, "/order")).toBe("/x-api/order");
  expect(splitPath(ENDPOINT_PREFIX, "/x-api/order/{id}")).toBe("order/{id}");
  // Правило, заведённое мимо нашей формы, тоже должно открываться.
  expect(splitPath(ENDPOINT_PREFIX, "/v1/order")).toBe("v1/order");
});
