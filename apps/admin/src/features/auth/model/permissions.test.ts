import { expect, test } from "vitest";
import { allowsGlobalRight } from "./permissions";

test("права не приехали — не запрещаем", () => {
  // Кнопка, пропавшая из-за незагруженного ответа, читается как
  // сломанное приложение. Настоящую проверку делает сервер.
  expect(allowsGlobalRight(null, "menu_button")).toBe(true);
});

test("запись есть, поля в ней нет — запрещено", () => {
  // `false` в ответ не попадает: поля GlobalPermission с omitempty.
  // Поэтому «пришёл объект без menu_drag» значит именно «нельзя».
  expect(allowsGlobalRight({ menu_button: true }, "menu_drag")).toBe(false);
  expect(allowsGlobalRight({ menu_button: true }, "menu_button")).toBe(true);
});
