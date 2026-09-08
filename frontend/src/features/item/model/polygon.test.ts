import { expect, test } from "vitest";
import { parsePolygon, polygonCenter, polygonPoints } from "./polygon";

/*
 * Формы значений взяты из живых данных: старая админка писала в колонку
 * JSON геометрии Яндекс-карты, а он бывает и списком точек, и списком
 * контуров — смотря какой версией рисовали.
 */
const RING = "[[41.3,69.2],[41.3,69.3],[41.4,69.3]]";
const RINGS = "[[[41.3,69.2],[41.3,69.3],[41.4,69.3]]]";

test("разбирается и плоский список точек, и список контуров", () => {
  const expected = [
    { lat: 41.3, lon: 69.2 },
    { lat: 41.3, lon: 69.3 },
    { lat: 41.4, lon: 69.3 },
  ];

  expect(parsePolygon(RING)).toEqual(expected);
  // Внешний контур — первый; дырки внутри отбрасываются.
  expect(parsePolygon(RINGS)).toEqual(expected);
  // Уже разобранное значение тоже принимается: бэкенд отдаёт колонку
  // то строкой, то массивом.
  expect(parsePolygon(JSON.parse(RING))).toEqual(expected);
});

test("мусор в колонке не становится областью", () => {
  expect(parsePolygon("не json")).toEqual([]);
  expect(parsePolygon("[]")).toEqual([]);
  expect(parsePolygon(null)).toEqual([]);
  // Точка вне Земли — не точка, но остальные сохраняются.
  expect(parsePolygon("[[41.3,69.2],[500,700],[41.4,69.3]]")).toHaveLength(2);
});

test("центр — середина рамки", () => {
  expect(polygonCenter(parsePolygon(RING))).toEqual({ lat: 41.35, lon: 69.25 });
  expect(polygonCenter([])).toBe(null);
});

test("миниатюра вписана в квадрат и не перевёрнута", () => {
  const points = polygonPoints(parsePolygon(RING), 100)
    .split(" ")
    .map((pair) => pair.split(",").map(Number));

  expect(points).toHaveLength(3);

  for (const [x, y] of points) {
    expect(x).toBeGreaterThanOrEqual(0);
    expect(x).toBeLessThanOrEqual(100);
    expect(y).toBeGreaterThanOrEqual(0);
    expect(y).toBeLessThanOrEqual(100);
  }

  // Самая северная точка (41.4) рисуется выше южных: у svg ось вниз.
  const north = points[2]![1]!;
  expect(north).toBeLessThan(points[0]![1]!);
});

test("одна точка не делит на ноль", () => {
  expect(polygonPoints([{ lat: 41.3, lon: 69.2 }], 100)).toBe("50,50");
});
