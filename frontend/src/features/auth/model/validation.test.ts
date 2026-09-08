import { expect, test } from "vitest";
import { checkPassword, isLoginValid, isPasswordValid } from "./validation";

/**
 * Правила списаны с бэкенда (pkg/util/utils.go ValidStrongPassword).
 * Тест ловит расхождение: если здесь ослабить проверку, пользователь
 * получит отказ сервера вместо подсказки в форме.
 */
test("пароль требует длину, цифру, строчную и заглавную", () => {
  expect(checkPassword("Passw0rd")).toEqual({
    length: true,
    digit: true,
    lower: true,
    upper: true,
  });

  expect(isPasswordValid("Passw0rd")).toBe(true);
  expect(isPasswordValid("passw0rd")).toBe(false); // нет заглавной
  expect(isPasswordValid("PASSW0RD")).toBe(false); // нет строчной
  expect(isPasswordValid("Password")).toBe(false); // нет цифры
  expect(isPasswordValid("Pas0w")).toBe(false); // 5 символов
});

test("логин от 6 символов, пробелы не считаются", () => {
  expect(isLoginValid("nurmuhammad")).toBe(true);
  expect(isLoginValid("abcdef")).toBe(true);
  expect(isLoginValid("abcde")).toBe(false);
  expect(isLoginValid("   abc   ")).toBe(false);
});
