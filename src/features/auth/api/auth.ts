import { useMutation } from "@tanstack/react-query";
import { ApiError, authApi, setRefreshHandler } from "@/shared/api/client";
import { session } from "@/shared/api/session";
import type {
  Connection,
  ConnectionSelection,
  Credentials,
  LoginContext,
  LoginResult,
  Registration,
} from "../model/types";
import type {
  ConnectionDto,
  DefaultLoginDto,
  LoginResponseDto,
  RegisterCompanyDto,
} from "./dto";
import { storePermissions } from "../model/permissions";
import { toConnection, toPermission, toSession } from "./normalize";

/**
 * Единственное место в приложении, где объявлены запросы авторизации.
 * Версия API — в пути, а не в baseURL.
 */

const DEFAULT_LOGIN = "/v3/multicompany/default-login";
const LOGIN = "/v2/login";
const REFRESH = "/v2/refresh";
const REGISTER_COMPANY = "/company";

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
 * Второй шаг: выбранные записи превращаются в tables и отправляются
 * в /v2/login. Одна запись на каждую connection.
 */
async function loginWithConnections(input: {
  credentials: Credentials;
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

export function useLogin() {
  return useMutation({ mutationFn: login });
}

export function useLoginWithGoogle() {
  return useMutation({ mutationFn: loginWithGoogle });
}

export function useRegister() {
  return useMutation({ mutationFn: register });
}

export function useLoginWithConnections() {
  return useMutation({ mutationFn: loginWithConnections });
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
