import { useQuery } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import { keys } from "@/shared/lib/query-keys";

/**
 * Функции проекта — то, что зовёт поле-кнопка (тип BUTTON).
 *
 * Нужны имя и ТИП: настройка кнопки хранит id (attributes.function),
 * человеку показывать надо имя, а тип решает, спрашивать ли путь —
 * у функции-процесса (WORKFLOW) вызов идёт по нему, и без пути
 * действие не запускается. Остальное, что отдаёт ручка — папка, ветка,
 * статус сборки, — здесь не при чём.
 *
 * Список общий на окружение, поэтому и ключ такой: у панели настроек
 * поля своего среза нет, а перезапрашивать его на каждое открытие
 * панели незачем — функции заводят в другом разделе и редко.
 */
const FUNCTIONS_STALE = 5 * 60_000;

type FunctionDto = { id?: string; name?: string; type?: string };
type FunctionsResponse = { functions?: FunctionDto[] };

export type ProjectFunction = { id: string; name: string; type: string };

/** Функция-процесс: у неё вызов идёт по пути, а не по одному id. */
export const WORKFLOW = "WORKFLOW";

export function useFunctions() {
  const session = useSession();
  const envId = session.getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.functions.list(envId),
    queryFn: () => api.get<FunctionsResponse>("/v2/function"),
    staleTime: FUNCTIONS_STALE,
  });

  return {
    functions: toFunctions(query.data),
    isLoading: query.isLoading,
  };
}

/**
 * Безымянную функцию показываем её id: список выбора, где половина
 * строк пустая, не выбирается вовсе.
 */
export function toFunctions(data: FunctionsResponse | undefined): ProjectFunction[] {
  return (data?.functions ?? [])
    .filter((item): item is FunctionDto & { id: string } => Boolean(item.id))
    .map((item) => ({
      id: item.id,
      name: item.name?.trim() || item.id,
      type: item.type?.trim() ?? "",
    }));
}
