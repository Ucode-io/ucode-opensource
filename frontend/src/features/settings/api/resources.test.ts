import { expect, test } from "vitest";
import { toResource, toSettings, toVariables } from "./resources";

/*
 * Переменные приезжают из JSON_AGG по LEFT JOIN (`resource.go:1725`).
 * Когда их нет, ответ содержит не пустой список, а одну строку из
 * null-ов; до нас она доезжает пустым объектом (пустые строки шлюз
 * выбрасывает через omitempty) — и `variables.length` на ней врёт.
 */
test("пустой список переменных не превращается в одну пустую", () => {
  const resource = toResource({ id: "r1", type: "REST", variables: [{}] });

  expect(resource.variables).toEqual([]);
});

test("настройки читаются из своего конверта, а чужие поля не подбираются", () => {
  const resource = toResource({
    id: "r1",
    name: " Playmobile ",
    type: "SMS",
    settings: {
      sms: { login: "ucode", number_of_otp: 6, service_type: "playmobile" },
      smtp: { email: "чужое@example.com" },
    },
  });

  expect(resource.name).toBe("Playmobile");
  expect(resource.settings.login).toBe("ucode");
  // Число приезжает числом, а правят его текстом.
  expect(resource.settings.number_of_otp).toBe("6");
  // Поля, которого у типа нет, в форме тоже нет.
  expect(resource.settings.service_type).toBeUndefined();
  expect(resource.settings.email).toBeUndefined();
  // Не заполнено — пустая строка, а не undefined: это значение поля ввода.
  expect(resource.settings.password).toBe("");
});

/*
 * `number_of_otp` в proto — int32. Строка в нём валит разбор ВСЕГО тела,
 * а не одного поля, поэтому пустое поле должно уехать нулём.
 */
test("числовое поле уезжает числом, пустое — нулём", () => {
  const body = toSettings("SMS", {
    login: " ucode ",
    password: "secret",
    originator: "3700",
    number_of_otp: "6",
    default_otp: "",
  });

  expect(body).toEqual({
    sms: {
      login: "ucode",
      password: "secret",
      originator: "3700",
      number_of_otp: 6,
      default_otp: "",
    },
  });

  expect(toSettings("SMS", { number_of_otp: "" })).toMatchObject({
    sms: { number_of_otp: 0 },
  });
});

test("тип без настроек не отправляет пустой конверт", () => {
  expect(toSettings("REST", {})).toBeUndefined();
  expect(toSettings("TRANSCODER", {})).toBeUndefined();
});

/*
 * Пустой id — признак новой переменной (`resource.go:2147`). Отправить
 * его строкой значит попросить обновить строку с id «».
 */
test("новая переменная уезжает без id, безымянная — не уезжает вовсе", () => {
  expect(
    toVariables([
      { id: "", key: " base_url ", value: "https://example.com" },
      { id: "v2", key: "token", value: "secret" },
      { id: "", key: "   ", value: "мусор" },
    ]),
  ).toEqual([
    { key: "base_url", value: "https://example.com" },
    { id: "v2", key: "token", value: "secret" },
  ]);
});
