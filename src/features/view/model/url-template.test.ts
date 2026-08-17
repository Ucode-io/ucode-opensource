import { expect, test } from "vitest";
import { fillTemplate, fillUrl, isExternal, toUrlTemplate } from "./url-template";

const ROW = { guid: "7f3", status: "в работе", empty: null };

test("переменная подставляется значением поля и кодируется", () => {
  expect(fillTemplate("/order/{{$guid}}", ROW)).toBe("/order/7f3");
  // Пробел в значении иначе разломал бы адрес.
  expect(fillTemplate("/s/{{$status}}", ROW)).toBe("/s/%D0%B2%20%D1%80%D0%B0%D0%B1%D0%BE%D1%82%D0%B5");
});

test("неизвестная и пустая переменные исчезают, а не остаются скобками", () => {
  expect(fillTemplate("/order/{{$nope}}", ROW)).toBe("/order/");
  expect(fillTemplate("/order/{{$empty}}", ROW)).toBe("/order/");
});

test("параметры уезжают строкой запроса, пустой ключ отбрасывается", () => {
  const url = fillUrl(
    {
      url: "/report",
      params: [
        { key: "id", value: "{{$guid}}" },
        { key: "", value: "{{$status}}" },
      ],
    },
    ROW,
  );

  expect(url).toBe("/report?id=7f3");
});

test("свой знак вопроса в адресе не задваивается", () => {
  const url = fillUrl({ url: "/report?year=2026", params: [{ key: "id", value: "{{$guid}}" }] }, ROW);

  expect(url).toBe("/report?year=2026&id=7f3");
});

test("внешним считается адрес по протоколу, а не по первому символу", () => {
  expect(isExternal("https://example.com/a")).toBe(true);
  expect(isExternal("//example.com/a")).toBe(true);
  expect(isExternal("/orders/7f3")).toBe(false);
});

/*
 * Настройки старой админки пишут объект, а её же экраны читают
 * `attributes.url_object` как строку — в проектах лежит и то, и другое.
 */
test("адрес читается и объектом, и голой строкой", () => {
  expect(toUrlTemplate("/orders")).toEqual({ url: "/orders", params: [] });
  expect(toUrlTemplate({ url: "/orders", params: [{ key: "a", value: "b" }] })).toEqual({
    url: "/orders",
    params: [{ key: "a", value: "b" }],
  });
  expect(toUrlTemplate(undefined)).toEqual({ url: "", params: [] });
});
