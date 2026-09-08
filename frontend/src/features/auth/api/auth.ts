import { useMutation } from "@tanstack/react-query";
import { ApiError, authApi, setRefreshHandler } from "@/shared/api/client";
import { session } from "@/shared/api/session";
import type {
  Connection,
  ConnectionSelection,
  Credentials,
  Invite,
  LoginContext,
  LoginResult,
  OtpChannel,
  OtpCredentials,
  RecoveryStart,
  Registration,
} from "../model/types";
import type {
  ConnectionDto,
  DefaultLoginDto,
  ForgotPasswordDto,
  LoginResponseDto,
  RegisterCompanyDto,
  SendCodeDto,
  VerifyEmailDto,
} from "./dto";
import { storePermissions } from "../model/permissions";
import { toConnection, toPermission, toRecoveryStart, toSession } from "./normalize";

/**
 * Единственное место в приложении, где объявлены запросы авторизации.
 * Версия API — в пути, а не в baseURL.
 */

const DEFAULT_LOGIN = "/v3/multicompany/default-login";
const LOGIN = "/v2/login";
const SEND_CODE_APP = "/v2/send-code-app";
const REFRESH = "/v2/refresh";
const REGISTER_COMPANY = "/company";
const REGISTER_USER = "/v2/register";
const FORGOT_PASSWORD = "/v2/forgot-password";
const SET_EMAIL = "/v2/set-email/send-code";
const VERIFY_EMAIL = "/v2/verify-only-email";
const RESET_PASSWORD = "/v2/reset-password";

function storeTokens(dto: LoginResponseDto, projectId: string, companyName = "") {
  if (!dto.token) throw new Error("Бэкенд не вернул токен");

  session.set({
    access: dto.token.access_token,
    refresh: dto.token.refresh_token,
    ...(dto.environment_id ? { environmentId: dto.environment_id } : {}),
    ...(projectId ? { projectId } : {}),
    ...(dto.user_id ? { userId: dto.user_id } : {}),
  });

  // Подписи для шапки сайдбара приходят прямо в ответе логина —
  // отдельного запроса за профилем не нужно.
  const name = dto.user?.name || dto.user?.login || dto.user?.email || "";
  if (name || dto.role?.name || companyName) {
    session.setProfile({
      name,
      role: dto.role?.name ?? "",
      company: companyName,
    });
  }

  /*
   * Права роли приезжают тем же ответом — и у логина, и у обновления
   * токена (V2LoginResponse у обоих). Отдельной ручки за ними нет.
   *
   * Пустой список НЕ записываем: это «ответ без прав», а не «прав нет»,
   * и запись затёрла бы то, что уже лежит, — панель настроек исчезла бы
   * после первого же обновления токена.
   */
  if (dto.permissions?.length) storePermissions(dto.permissions.map(toPermission));

  /*
   * Глобальные права роли — тем же ответом. Пустой объект записываем
   * тоже: у роли, которой всё запрещено, в ответе останется один `id`,
   * и пропустить его значило бы оставить старые права от прошлой роли.
   */
  if (dto.global_permission) session.setGlobalRights(dto.global_permission);
}

async function login(credentials: Credentials): Promise<LoginResult> {
  const data = await authApi.post<DefaultLoginDto>(DEFAULT_LOGIN, {
    ...credentials,
    type: "default",
  });

  return interpretLogin(data);
}

/** Разбор ответа входа — один на пароль и на Google. */
function interpretLogin(data: DefaultLoginDto): LoginResult {
  // Форма ответа зависит от данных: массив connection'ов, если войти можно
  // больше чем одним способом, иначе готовая сессия. Поэтому проверка
  // здесь, а не в типе.
  if (Array.isArray(data.response)) {
    return {
      kind: "choose-connections",
      connections: (data.response as ConnectionDto[]).map(toConnection),
      context: {
        clientTypeId: data.client_type ?? "",
        projectId: data.project ?? data.project_data?.project_id ?? "",
        environmentId: data.environment ?? "",
      },
    };
  }

  const response = data.response;
  if (!response) throw new Error("Пустой ответ логина");

  const projectId = data.project_data?.project_id ?? "";
  const company = data.project_data?.title ?? data.project_data?.name ?? "";
  storeTokens(response, projectId, company);

  return { kind: "session", session: { ...toSession(response), projectId } };
}

/**
 * Вход по коду, шаг 1: код уходит в SMS или письмом. Возвращается
 * sms_id — он же уезжает вторым шагом вместе с кодом. Заодно ручка
 * проверяет, что пользователь с таким телефоном или почтой существует
 * (register_v2.go, V2SendCodeApp).
 *
 * Ручка одна на оба канала, различает их `type`; формат получателя она
 * же и проверяет: `+` и двенадцать цифр у телефона (util.IsValidPhone),
 * обычная почта у письма.
 */
async function sendCode(input: { recipient: string; channel: OtpChannel }): Promise<string> {
  const data = await authApi.post<SendCodeDto>(SEND_CODE_APP, {
    recipient: input.recipient,
    type: input.channel,
  });

  if (!data.sms_id) throw new Error("Бэкенд не вернул sms_id");
  return data.sms_id;
}

/**
 * Вход по коду, шаг 2: тот же default-login, что и у пароля, — код
 * проверяется в нём (authenticateUser, случаи WithPhone и WithEmail),
 * и дальше всё общее: connection'ы или готовая сессия.
 */
async function loginWithOtp(credentials: OtpCredentials): Promise<LoginResult> {
  const data = await authApi.post<DefaultLoginDto>(DEFAULT_LOGIN, credentials);
  return interpretLogin(data);
}

/**
 * Второй шаг: выбранные записи превращаются в tables и отправляются
 * в /v2/login. Одна запись на каждую connection.
 *
 * `credentials` — то, чем входили на первом шаге: пароль или телефон
 * с кодом. /v2/login проверяет их заново, поэтому payload тот же.
 */
async function loginWithConnections(input: {
  credentials: Credentials | OtpCredentials;
  context: LoginContext;
  connections: Connection[];
  selection: ConnectionSelection;
}): Promise<LoginResult> {
  const tables = input.connections
    .filter((connection) => input.selection[connection.id])
    .map((connection) => ({
      table_slug: connection.tableSlug,
      object_id: input.selection[connection.id],
    }));

  const response = await authApi.post<LoginResponseDto>(
    LOGIN,
    {
      ...input.credentials,
      tables,
      client_type: input.context.clientTypeId,
      project_id: input.context.projectId,
      environment_id: input.context.environmentId,
    },
    { headers: { "Environment-Id": input.context.environmentId } },
  );

  storeTokens(response, input.context.projectId);

  return {
    kind: "session",
    session: { ...toSession(response), projectId: input.context.projectId },
  };
}

/**
 * Вход через Google. Отправляем OAuth access token: бэкенд проверяет его
 * запросом в googleapis.com/oauth2/v3/userinfo и читает email оттуда
 * (session_service_v2.go, случай WithGoogle). ID token сюда не подходит —
 * его этот эндпоинт не примет.
 */
async function loginWithGoogle(accessToken: string): Promise<LoginResult> {
  const data = await authApi.post<DefaultLoginDto>(DEFAULT_LOGIN, {
    type: "google",
    google_token: accessToken,
  });

  return interpretLogin(data);
}

/**
 * Регистрация компании вместе с её первым пользователем.
 *
 * Только email и пароль. Регистрация через Google не реализована
 * сознательно: бэкенд (company_service.go:78) декодирует Google-токен
 * без проверки подписи, поэтому подделанный токен создаёт компанию
 * на любой чужой email. Вернём, когда там появится проверка подписи.
 */
async function register(input: Registration): Promise<void> {
  const body: RegisterCompanyDto = {
    name: input.companyName,
    user_info: {
      login: input.login,
      email: input.email,
      password: input.password,
    },
  };

  await authApi.post<unknown>(REGISTER_COMPANY, body);
}

/**
 * Восстановление пароля — четыре шага, и все четыре у бэкенда свои:
 *
 *   1. forgot-password        нашли логин, отправили код на почту;
 *   2. set-email/send-code    почты у пользователя не было — задаём её
 *                             и отправляем код уже туда;
 *   3. verify-only-email      проверяем код;
 *   4. reset-password         записываем новый пароль.
 *
 * Токена ни один из них не требует: мидлвар auth-сервиса пропускает
 * запрос без заголовка Authorization (handlers/middleware.go:41).
 */
async function startRecovery(login: string): Promise<RecoveryStart> {
  const data = await authApi.post<ForgotPasswordDto>(FORGOT_PASSWORD, { login: login.trim() });
  return toRecoveryStart(data);
}

async function sendCodeToEmail(input: { userId: string; email: string }): Promise<RecoveryStart> {
  const data = await authApi.put<ForgotPasswordDto>(SET_EMAIL, {
    user_id: input.userId,
    email: input.email.trim(),
  });

  return {
    kind: "sent",
    userId: data.user_id || input.userId,
    smsId: data.sms_id ?? "",
    email: data.email || input.email.trim(),
  };
}

/**
 * Проверка кода из письма. `register_type` бэкенд требует, и для
 * восстановления это всегда «default» — так его шлёт и старая админка.
 *
 * Ответ `verified: false` — это не ошибка запроса, а неверный код,
 * поэтому наружу отдаётся булево, а не исключение.
 */
async function verifyCode(input: { smsId: string; otp: string }): Promise<boolean> {
  const data = await authApi.post<VerifyEmailDto>(VERIFY_EMAIL, {
    sms_id: input.smsId,
    otp: input.otp.trim(),
    register_type: "default",
  });

  return data.verified === true;
}

async function setPassword(input: { userId: string; password: string }): Promise<void> {
  await authApi.put<unknown>(RESET_PASSWORD, {
    user_id: input.userId,
    password: input.password,
  });
}

/**
 * Регистрация по приглашению: пользователь заводится сразу в проекте,
 * с ролью из ссылки, и тут же входит.
 *
 * Тело — `{data: {...}}`: бэкенд читает из него `type`, `client_type_id`
 * и `role_id` приведением типа без проверки (register_v2.go:408), и
 * запрос без любого из них роняет обработчик. Проект и окружение
 * кладём туда же, а не только в адрес и заголовок: из тела он берёт их
 * первыми, а адрес читает лишь как запасной вариант.
 *
 * Вход после регистрации — через /v2/login с идентификаторами проекта:
 * приглашённый пользователь живёт в конкретном окружении, и
 * multicompany-вход про него ничего не знает.
 */
async function acceptInvite(input: {
  credentials: Credentials;
  invite: Invite;
}): Promise<LoginResult> {
  const { invite } = input;
  const login = input.credentials.username.trim();

  await authApi.post<unknown>(
    REGISTER_USER,
    {
      data: {
        type: "login",
        login,
        password: input.credentials.password,
        role_id: invite.roleId,
        client_type_id: invite.clientTypeId,
        project_id: invite.projectId,
        environment_id: invite.environmentId,
      },
    },
    {
      params: { "project-id": invite.projectId },
      headers: { "Environment-Id": invite.environmentId },
    },
  );

  const response = await authApi.post<LoginResponseDto>(
    LOGIN,
    {
      username: login,
      password: input.credentials.password,
      tables: [],
      client_type: invite.clientTypeId,
      project_id: invite.projectId,
      environment_id: invite.environmentId,
    },
    { headers: { "Environment-Id": invite.environmentId } },
  );

  storeTokens(response, invite.projectId);

  return {
    kind: "session",
    session: { ...toSession(response), projectId: invite.projectId },
  };
}

export function useLogin() {
  return useMutation({ mutationFn: login });
}

export function useLoginWithGoogle() {
  return useMutation({ mutationFn: loginWithGoogle });
}

export function useSendCode() {
  return useMutation({ mutationFn: sendCode });
}

export function useLoginWithOtp() {
  return useMutation({ mutationFn: loginWithOtp });
}

export function useRegister() {
  return useMutation({ mutationFn: register });
}

export function useLoginWithConnections() {
  return useMutation({ mutationFn: loginWithConnections });
}

export function useStartRecovery() {
  return useMutation({ mutationFn: startRecovery });
}

export function useSendCodeToEmail() {
  return useMutation({ mutationFn: sendCodeToEmail });
}

export function useVerifyCode() {
  return useMutation({ mutationFn: verifyCode });
}

export function useSetPassword() {
  return useMutation({ mutationFn: setPassword });
}

export function useAcceptInvite() {
  return useMutation({ mutationFn: acceptInvite });
}

export function logout() {
  session.clear();
}

/**
 * Обновление токена. Вызывается только клиентом при 401 — в одном
 * экземпляре на всё приложение, см. shared/api/client.
 */
export function installRefreshHandler() {
  setRefreshHandler(async () => {
    const refresh = session.getRefresh();
    if (!refresh) return false;

    try {
      const data = await authApi.put<LoginResponseDto>(
        REFRESH,
        {
          refresh_token: refresh,
          project_id: session.getProjectId() ?? "",
          env_id: session.getEnvironmentId() ?? "",
        },
        // Мимо перехватчика: иначе отказ обновления запустил бы
        // обновление же и приложение зависло бы.
        { skipAuthRefresh: true },
      );

      if (!data.token) return false;
      storeTokens(data, session.getProjectId() ?? "");
      return true;
    } catch (error) {
      // Сессию сносим ТОЛЬКО когда сервер сказал «этот refresh больше
      // не годится». Сетевой сбой, 500 или таймаут — это не повод
      // выкидывать человека: раньше любая такая ошибка означала логаут.
      const rejected = error instanceof ApiError && (error.status === 401 || error.status === 403);
      if (rejected) session.clear();
      return false;
    }
  });
}
