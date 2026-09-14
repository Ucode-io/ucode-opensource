import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { errorMessage, reportError, toast } from "@/shared/lib/toast";

/**
 * Выполнение SQL прямо в базе проекта.
 *
 * Одна ручка и одна мутация: запрос уезжает, ответ приходит целиком.
 * Кэша здесь нет и быть не должно — «показать то же, что в прошлый раз»
 * для запроса, который человек только что изменил, значит соврать.
 *
 * Работает только на postgres-проекте: шлюз отказывает всему остальному
 * до похода в базу — `resolveAiChatService` (`ai_chat.go:146`) отвечает
 * «resource type not supported». Ресурс берётся по проекту И окружению,
 * то есть запрос уходит в базу ТЕКУЩЕГО окружения, а не проекта вообще.
 *
 * Про то, почему ошибку SQL здесь не показать, — `useRunSql` ниже
 * и docs/backend-notes.md, «SQL-консоль».
 */
const EXEC_QUERY = "/v1/custom-endpoints/exec-query";

export type SqlResult = {
  /**
   * Колонки ответа. Порядок НЕ тот, что в запросе: строки приезжают
   * объектами (`google.protobuf.Struct`, `pg_object_builder.proto:59`),
   * а у объекта порядка полей нет. Восстановить его неоткуда —
   * `SELECT b, a` вернёт `a, b`.
   */
  columns: string[];
  rows: Record<string, unknown>[];
  /** Тип колонки в postgres: `uuid`, `int4`, `timestamptz`, `unknown`. */
  types: Record<string, string>;
  /** Сколько строк вернулось или изменилось — считает сам бэкенд. */
  rowsAffected: number;
};

type ExecDto = {
  rows?: (Record<string, unknown> | null)[] | null;
  rows_affected?: number;
  types?: Record<string, string> | null;
};

/**
 * Запустить запрос.
 *
 * Ошибку разбитого SQL сюда НЕ приносит: база отвечает текстом ошибки
 * в поле `error` (`object_builder.go:3393`), но шлюз это поле
 * выбрасывает — в ответ уходят только `rows`, `rows_affected` и `types`
 * (`custom-endpoints.go:36-40`), и код ответа остаётся 201. Соседняя
 * ручка того же файла (`RunCustomEndpoint`, `:223`) поле читает
 * и отвечает 400 — то есть это недосмотр, а не решение.
 *
 * Поэтому «пусто» здесь двусмысленно, и разбирает эту двусмысленность
 * не этот файл, а экран: он единственный может сказать про неё словами.
 * Здесь остаётся честная нормализация — пусто значит пусто.
 */
export function useRunSql() {
  return useMutation({
    mutationFn: async (sql: string): Promise<SqlResult> => {
      const dto = await api.post<ExecDto>(EXEC_QUERY, { sql });

      const rows = (dto.rows ?? []).filter((row): row is Record<string, unknown> => Boolean(row));
      const types = dto.types ?? {};

      /*
       * Колонки — из первой строки, а когда строк нет, из типов: их
       * бэкенд собирает по описанию полей ответа (`rows.FieldDescriptions()`),
       * поэтому у SELECT без единой строки заголовки всё равно есть.
       */
      return {
        columns: Object.keys(rows[0] ?? types),
        rows,
        types,
        rowsAffected: dto.rows_affected ?? 0,
      };
    },
  });
}

/**
 * Сохранённые запросы — тот же SQL, оставленный на потом.
 *
 * Один и тот же адрес (`/v1/custom-endpoints`) хранит текст запроса
 * в базе проекта и умеет выполнить его по идентификатору
 * (`POST /:id/run`, `api.go:535`). В консоли это список слева: написал,
 * сохранил, вернулся через неделю — вместо того чтобы держать разбор
 * непонятной таблицы в заметках.
 *
 * На бэкенде они называются custom endpoints, здесь — SavedQuery:
 * слово `Endpoint` в этой же фиче уже занято правилами подмены пути
 * (`model/endpoint`), и два разных «эндпоинта» в одной папке — это
 * ровно та путаница, ради которой заведён CONTEXT.md.
 */
const QUERIES = "/v1/custom-endpoints";

export type SavedQuery = { id: string; name: string; sql: string };

type QueryDto = { id?: string; name?: string; sql?: string };

/**
 * Список. Сотня разом и без страниц: `count` в ответе — это длина
 * ТЕКУЩЕЙ страницы (`storage/postgres/custom_endpoint.go:206` —
 * `len(endpoints)`), а не общее число, поэтому нарисовать страницы
 * всё равно нечем. Сохранённых запросов в проекте единицы; упрёмся
 * в сотню — придётся сначала чинить счётчик на бэкенде.
 *
 * Порядок — какой отдаёт ручка (`created_at DESC`): только что
 * сохранённый оказывается сверху, там же, куда смотрят.
 */
export function useSavedQueries() {
  const envId = useSession().getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.sqlQueries(envId),
    queryFn: () => api.get<{ endpoints?: QueryDto[] }>(QUERIES, { params: { limit: 100 } }),
    enabled: Boolean(envId),
    staleTime: 60_000,
    select: (data): SavedQuery[] =>
      (data.endpoints ?? [])
        .filter((dto) => dto.id)
        .map((dto) => ({ id: dto.id!, name: dto.name?.trim() || dto.id!, sql: dto.sql ?? "" })),
  });

  /*
   * Отказ показывается словами, а не пустым списком: таблица
   * `custom_endpoint` появилась в базе проекта отдельной миграцией
   * (`migrations/postgres/000050`), и у базы, которую не докатили,
   * ручка отвечает ошибкой. «Сохранённых запросов пока нет» на этом
   * месте означало бы «сохраняй сюда», а сохранить будет некуда.
   */
  return {
    queries: query.data ?? NO_QUERIES,
    isLoading: query.isLoading,
    error: errorMessage(query.error, "sql.loadFailed"),
  };
}

const NO_QUERIES: SavedQuery[] = [];

/**
 * Сохранить написанное. Спрашиваем только имя — остальные поля ручки
 * сегодня ничего не решают, и форма их не изображает:
 *
 * - `method` уходит постоянным «POST». Запускается сохранённый запрос
 *   всегда одной ручкой `POST /v1/custom-endpoints/:id/run`, и по этому
 *   полю не расходится ни один маршрут — во всём бэкенде его никто
 *   не читает. Выпадающий список из четырёх глаголов означал бы выбор,
 *   которого нет.
 * - `in_transaction` не шлём вовсе: колонка есть, но `Run` её не читает
 *   (docs/backend-notes.md, «SQL-консоль»), — галка врала бы.
 * - `parameters` — пустой список: именованные `:параметры` бэкенд
 *   находит регуляркой в самом тексте запроса
 *   (`custom_endpoint.go:283`), а объявленный список нужен только
 *   для `$1`-стиля, которого в консоли не написать (postgres не примет
 *   `$1` без аргументов, и до сохранения такой запрос не дожил бы).
 */
export function useSaveQuery() {
  const invalidate = useInvalidateQueries();

  return useMutation({
    mutationFn: async ({ name, sql }: { name: string; sql: string }): Promise<SavedQuery> => {
      const dto = await api.post<QueryDto>(QUERIES, {
        name: name.trim(),
        sql,
        method: "POST",
        parameters: [],
      });

      return { id: dto.id ?? "", name: dto.name ?? name.trim(), sql: dto.sql ?? sql };
    },
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("sql.saved"));
      await invalidate();
    },
  });
}

/**
 * Переписать текст сохранённого запроса. Уезжает ОДНО поле: правка
 * собирает `SET` только из непустых значений (`custom_endpoint.go:88-118`),
 * поэтому имя, описание и параметры остаются прежними сами собой —
 * их не надо ни читать, ни слать обратно.
 */
export function useUpdateQuery() {
  const invalidate = useInvalidateQueries();

  return useMutation({
    mutationFn: ({ id, sql }: { id: string; sql: string }) =>
      api.put<unknown>(`${QUERIES}/${id}`, { sql }),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("sql.saved"));
      await invalidate();
    },
  });
}

export function useDeleteQuery() {
  const invalidate = useInvalidateQueries();

  return useMutation({
    mutationFn: (id: string) => api.delete<unknown>(`${QUERIES}/${id}`),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("sql.deleted"));
      await invalidate();
    },
  });
}

function useInvalidateQueries() {
  const queryClient = useQueryClient();
  const envId = useSession().getEnvironmentId() ?? "";

  return () => queryClient.invalidateQueries({ queryKey: keys.settings.sqlQueries(envId) });
}
