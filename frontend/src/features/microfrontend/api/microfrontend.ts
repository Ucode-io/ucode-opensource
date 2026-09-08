import { useQuery } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import { keys } from "@/shared/lib/query-keys";

/**
 * Микрофронтенд по идентификатору с пункта меню.
 *
 * Адрес сборки лежит в `url` — это поле 9 в `Function`
 * (`new_function_service/new_function.pb.go:275`). Обработчик шлюза
 * `GetMicroFrontEndByID` — прокси в функциональный сервис
 * (`api/handlers/v1/microfrontend.go:49`), поэтому swagger-аннотация
 * над ним ничего о теле не говорит: истина в proto.
 */
type MicrofrontendDto = {
  id?: string;
  name?: string;
  /** Голый хост, без схемы: `my-app.example.com`. */
  url?: string;
  type?: string;
};

export type Microfrontend = {
  id: string;
  name: string;
  url: string;
};

/**
 * Микрофронтенды проекта — из них выбирают, что покажет пункт меню.
 *
 * Ответ приходит под ключом `functions`: микрофронтенд для этого
 * сервиса — разновидность функции, отсюда и `/v2/functions/…`
 * в адресе (так его читает и старая админка,
 * `MicrofrontendLinkModal.jsx:47`).
 */
export function useMicrofrontends(enabled: boolean) {
  const envId = useSession().getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: [...keys.microfrontends.all, envId, "list"] as const,
    queryFn: () => api.get<{ functions?: MicrofrontendDto[] }>("/v2/functions/micro-frontend"),
    enabled,
    staleTime: 5 * 60_000,
  });

  return {
    items: (query.data?.functions ?? [])
      .filter((dto): dto is MicrofrontendDto & { id: string } => Boolean(dto.id))
      .map((dto) => ({ id: dto.id, name: dto.name?.trim() || dto.id, url: dto.url?.trim() ?? "" })),
    isLoading: query.isLoading,
    error: query.error,
  };
}

export function useMicrofrontend(id: string) {
  const envId = useSession().getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.microfrontends.byId(envId, id),
    queryFn: () => api.get<MicrofrontendDto>(`/v2/functions/micro-frontend/${id}`),
    enabled: Boolean(id),
    // Адрес меняется, только когда ремоут пересобрали и продвинули.
    staleTime: 5 * 60_000,
  });

  const dto = query.data;

  return {
    microfrontend: dto
      ? ({ id: dto.id ?? id, name: dto.name ?? "", url: dto.url?.trim() ?? "" } as Microfrontend)
      : undefined,
    isLoading: query.isLoading,
    error: query.error,
  };
}
