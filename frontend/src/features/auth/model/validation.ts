/**
 * Правила бэкенда, продублированные на клиенте — чтобы пользователь узнал
 * о них до отправки формы, а не из ответа сервера на английском.
 *
 * Списаны с кода, а не придуманы:
 *   pkg/util/utils.go ValidStrongPassword — 6+ символов, заглавная,
 *     строчная, цифра;
 *   grpc/service/company_service.go — len(login) < 6 отбрасывается.
 * Если бэкенд ужесточит правила, здесь тоже надо поправить.
 */
export const LOGIN_MIN_LENGTH = 6;

export const PASSWORD_RULES = [
  { key: "length", test: (v: string) => v.length >= 6 },
  { key: "digit", test: (v: string) => /\d/.test(v) },
  { key: "lower", test: (v: string) => /[a-z]/.test(v) },
  { key: "upper", test: (v: string) => /[A-Z]/.test(v) },
] as const;

export type PasswordRuleKey = (typeof PASSWORD_RULES)[number]["key"];

export function checkPassword(value: string): Record<PasswordRuleKey, boolean> {
  return Object.fromEntries(PASSWORD_RULES.map((r) => [r.key, r.test(value)])) as Record<
    PasswordRuleKey,
    boolean
  >;
}

export const isPasswordValid = (value: string) => PASSWORD_RULES.every((r) => r.test(value));
export const isLoginValid = (value: string) => value.trim().length >= LOGIN_MIN_LENGTH;
