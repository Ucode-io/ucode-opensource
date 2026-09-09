import { expect, test, vi } from "vitest";
import {
  EMPTY_URL_TEMPLATE,
  fillTemplate,
  fillUrl,
  isExternal,
  openCreateUrl,
  openRowUrl,
  toUrlTemplate,
} from "./url-template";

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

/*
 * Переход по настройке — общий для всех экранов: таблицы, доски,
 * календаря, таймлайна и вкладки связи. Проверяем именно развилку
 * «настройка есть / настройки нет»: она решает, открывать карточку
 * или уходить на страницу проекта.
 */
test("без адреса переход не состоится и карточку никто не отменит", () => {
  const opened: string[] = [];
  vi.stubGlobal("window", { open: (url: string) => opened.push(url) });

  expect(openRowUrl({ navigate: EMPTY_URL_TEMPLATE }, { guid: "1" })).toBe(false);
  expect(openCreateUrl({ objectUrl: EMPTY_URL_TEMPLATE })).toBe(false);
  expect(opened).toEqual([]);

  vi.unstubAllGlobals();
});

test("адрес строки подставляет её значения", () => {
  const opened: string[] = [];
  vi.stubGlobal("window", { open: (url: string) => opened.push(url) });

  const view = { navigate: toUrlTemplate({ url: "https://crm.example/{{$guid}}", params: [] }) };

  expect(openRowUrl(view, { guid: "7f3" })).toBe(true);
  expect(opened).toEqual(["https://crm.example/7f3"]);

  vi.unstubAllGlobals();
});

test("у создания подставлять нечего: строки ещё нет", () => {
  const opened: string[] = [];
  vi.stubGlobal("window", { open: (url: string) => opened.push(url) });

  const view = {
    objectUrl: toUrlTemplate({ url: "https://crm.example/new", params: [{ key: "from", value: "admin" }] }),
  };

  expect(openCreateUrl(view)).toBe(true);
  expect(opened).toEqual(["https://crm.example/new?from=admin"]);

  vi.unstubAllGlobals();
});
