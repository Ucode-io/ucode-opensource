export {
  installRefreshHandler,
  logout,
  useAcceptInvite,
  useLogin,
  useLoginWithConnections,
  useLoginWithGoogle,
  useRegister,
  useSendCodeToEmail,
  useSetPassword,
  useStartRecovery,
  useVerifyCode,
} from "./api/auth";
export {
  useGlobalRight,
  useIsSuperRole,
  useTablePermission,
  useTablePermissions,
} from "./model/permissions";
/*
 * Правила пароля живут здесь, но нужны не только входу: PASSWORD —
 * обычный тип поля, и строку с паролем заводят в карточке записи.
 * Проверка у бэкенда одна и та же (`ValidStrongPassword`), поэтому
 * и на клиенте она должна быть одна.
 */
export { PASSWORD_RULES, checkPassword, isPasswordValid } from "./model/validation";
export type { Permission } from "./model/types";
export { AuthLayout } from "./ui/AuthLayout";
export { InviteForm } from "./ui/InviteForm";
export { LoginForm } from "./ui/LoginForm";
export { RecoverForm } from "./ui/RecoverForm";
export { RegisterForm } from "./ui/RegisterForm";
export type {
  AuthSession,
  Connection,
  Credentials,
  Invite,
  LoginContext,
  RecoveryStart,
  Registration,
} from "./model/types";
