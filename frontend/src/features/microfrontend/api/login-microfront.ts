import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * Микрофронтенд на экране входа: проект подменяет форму логина своей.
 *
 * Привязка идёт к ПОДДОМЕНУ, а не к проекту: у одного проекта бывает
 * несколько адресов, и подменяется вход на конкретном
 * (`ProjectLoginMicroFrontend`, `projects_service.proto:111`).
 *
 * Чтение — единственная неавторизованная ручка во всей этой группе:
 * `GET /v1/login-microfront` висит вне группы с проверкой токена
 * (`api/api.go:39`), и иначе быть не могло — её зовут ДО входа.
 * Отсюда `skipAuthRefresh`: без токена шлюз ответит 403, и обновлять
 * тут нечего.
 *
 * Привязки нет — приходит не 404, а пустой объект (`login_microfront.go:140`).
 * Пустой `microfront_id` и есть «показывать нашу форму».
 */
export type LoginMicrofront = {
  /** id самой ПРИВЯЗКИ, не микрофронтенда. Нужен правке. */
  id: string;
  microfrontId: string;
  subdomain: string;
  /** Голый хост сборки. Пусто — показывается наша форма. */
  url: string;
};

type LoginMicrofrontDto = {
  id?: string;
  microfront_id?: string;
  subdomain?: string;
  function?: { url?: string };
};

const LOGIN_MICROFRONT = "/v1/login-microfront";

/**
 * Поддомен, на котором мы сейчас. Он же и есть ключ привязки.
 *
 * На localhost привязки не бывает: запись заводится под настоящий
 * адрес. Старая админка подставляла здесь ЗАШИТЫЙ чужой поддомен
 * (`router/NewRouter.jsx:123`) — из-за этого разработчик видел чужой
 * экран входа. Мы отдаём пустую строку: запрос просто не уходит.
 */
export function loginSubdomain(): string {
  const host = window.location.hostname;
  return host === "localhost" || host === "127.0.0.1" ? "" : host;
}

export function useLoginMicrofront(subdomain: string) {
  const query = useQuery({
    queryKey: keys.microfrontends.loginBinding(subdomain),
    queryFn: () =>
      api.get<LoginMicrofrontDto>(LOGIN_MICROFRONT, {
        params: { subdomain },
        skipAuthRefresh: true,
      }),
    enabled: Boolean(subdomain),
    /*
     * Один заход — один запрос. Экран входа показывается до всего
     * остального, и повтор при возврате фокуса перерисовывал бы его
     * посреди набора пароля.
     */
    staleTime: Infinity,
    retry: false,
    select: (dto): LoginMicrofront => ({
      id: dto.id ?? "",
      microfrontId: dto.microfront_id ?? "",
      subdomain: dto.subdomain ?? "",
      url: dto.function?.url?.trim() ?? "",
    }),
  });

  return { binding: query.data, isLoading: query.isLoading };
}

/**
 * Привязать микрофронтенд к поддомену или сменить привязанный.
 *
 * Ручек две, и выбор между ними — по наличию записи: `POST` заводит,
 * `PUT` правит. Своего «отвязать» у бэкенда нет вовсе, поэтому снятие
 * — это правка с пустым `microfront_id`: колонка обнуляется, и чтение
 * снова отдаёт пустоту.
 */
export function useBindLoginMicrofront() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      binding,
      microfrontId,
      subdomain,
    }: {
      /** Существующая привязка. Нет — заводим новую. */
      binding: LoginMicrofront | undefined;
      microfrontId: string;
      subdomain: string;
    }) =>
      binding?.id
        ? api.put<unknown>(LOGIN_MICROFRONT, {
            id: binding.id,
            microfront_id: microfrontId,
            subdomain,
          })
        : api.post<unknown>(LOGIN_MICROFRONT, {
            microfront_id: microfrontId,
            subdomain,
          }),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async (_data, { subdomain }) => {
      toast.success(i18n.t("settings.saved"));
      await queryClient.invalidateQueries({
        queryKey: keys.microfrontends.loginBinding(subdomain),
      });
    },
  });
}
