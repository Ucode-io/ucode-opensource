import { expect, test } from "vitest";
import { toTableSettings, toUpdateBody } from "./table-settings";

const DTO = {
  id: "1e1e",
  slug: "orders",
  label: "Заказы",
  icon: "shopping-cart",
  description: "Заказы клиентов",
  soft_delete: true,
  is_cached: false,
  order_by: false,
  is_login_table: false,
  increment_id: { with_increment_id: true, digit_number: 5 },
  attributes: { label_en: "Orders", label_cyr: "Буюртмалар", subtitle_field_slug: "client_id" },
};

test("настройки читаются колонками, а имена по языкам — из attributes", () => {
  const table = toTableSettings(DTO);

  expect(table).toMatchObject({
    id: "1e1e",
    slug: "orders",
    label: "Заказы",
    labels: { en: "Orders", cyr: "Буюртмалар" },
    softDelete: true,
    isCached: false,
    isLoginTable: false,
    loginStrategies: [],
  });
});

test("правка уходит поверх исходного ответа: чужие настройки остаются на месте", () => {
  const body = toUpdateBody({ table: toTableSettings(DTO), isCached: true });

  // UPDATE перезаписывает все колонки без условий: пропавший в теле
  // значок или инкрементный номер стёрся бы молча.
  expect(body).toMatchObject({
    id: "1e1e",
    slug: "orders",
    icon: "shopping-cart",
    increment_id: { with_increment_id: true, digit_number: 5 },
    is_cached: true,
    soft_delete: true,
  });
  expect(body.attributes).toMatchObject({ subtitle_field_slug: "client_id" });
});

test("имя пишется и по языкам, и колонкой label", () => {
  const body = toUpdateBody({
    table: toTableSettings(DTO),
    labels: { en: " Purchases ", cyr: "Буюртмалар" },
  });

  expect(body.label).toBe("Purchases");
  expect(body.attributes).toMatchObject({ label_en: "Purchases", label_cyr: "Буюртмалар" });
});

test("auth_info уходит только у таблицы входа и всегда со способами входа", () => {
  const table = toTableSettings(DTO);

  expect(toUpdateBody({ table }).attributes).not.toHaveProperty("auth_info");

  const body = toUpdateBody({ table, isLoginTable: true, loginStrategies: ["email"] });
  expect(body.is_login_table).toBe(true);
  expect(body.attributes).toMatchObject({ auth_info: { login_strategy: ["email"] } });
});

test("правка соседней настройки не трогает способы входа, даже незнакомые", () => {
  const table = toTableSettings({
    ...DTO,
    is_login_table: true,
    attributes: { ...DTO.attributes, auth_info: { login_strategy: ["esp", "email"] } },
  });

  const body = toUpdateBody({ table, isCached: true });
  const attributes = body.attributes as { auth_info: { login_strategy: string[] } };

  expect(attributes.auth_info.login_strategy).toEqual(["esp", "email"]);
});

test("незнакомый способ входа отбрасывается: бэкенд по нему поля не заведёт", () => {
  const table = toTableSettings({
    ...DTO,
    is_login_table: true,
    attributes: { ...DTO.attributes, auth_info: { login_strategy: ["email", "esp", "phone"] } },
  });

  expect(table.loginStrategies).toEqual(["email", "phone"]);
});
