/**
 * Договор микрофронтенда ucode, версия 1.
 *
 * Типы продублированы с хостом (`features/microfrontend/model/remote.ts`)
 * намеренно: хост не может зависеть от пакета, который сам же публикует.
 * Расходиться им нечему — договор заморожен номером `contract`, а лежат
 * они в одном репозитории и правятся одним коммитом.
 */

/** Что хост передаёт странице микрофронтенда. */
export type MicrofrontendProps = {
  /** Меняется при каждом открытии: ремоут перемонтируется, а не оживает. */
  activationKey: string;
  /** Тот же ключ вторым именем — так его читали ремоуты прежнего поколения. */
  microfrontendActivationKey: string;
  environment: "production" | "staging";
  /** Инстанс i18next хоста: подписи ремоута идут на языке админки. */
  i18n: unknown;
  /** Ось http админки — с токеном, окружением и обновлением. */
  sharedHttpRequest: unknown;
  /** Она же, но на `/v2`. */
  sharedHttpRequestV2: unknown;
  /** `attributes.params` пункта меню. */
  params: Record<string, string>;
};

/** Что хост передаёт ремоуту, подменяющему ЭКРАН ВХОДА. Договор другой. */
export type LoginProps = {
  loginAction: (credentials: { username: string; password: string }) => Promise<void>;
  i18n: unknown;
};

export type Meta = { contract: number; react: string };
export declare const meta: Meta;

type Request = <T = unknown>(url: string, ...rest: unknown[]) => Promise<T>;

export declare function createApi(props: Partial<MicrofrontendProps>): {
  get: Request;
  post: Request;
  put: Request;
  patch: Request;
  delete: Request;
  v2: { get: Request; post: Request; put: Request; patch: Request; delete: Request };
};
