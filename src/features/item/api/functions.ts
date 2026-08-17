import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";
import type { Item } from "../model/types";

/**
 * Вызов функции из поля-кнопки (тип BUTTON).
 *
 * Тело историческое, и три его места не читаются из имени:
 *
 *   function_id  id функции, а не путь. Лежит в attributes.function
 *                самого поля-кнопки
 *   object_data  СТРОКА ЦЕЛИКОМ, а не её guid: функция получает те же
 *                значения, что видит человек на экране, и не ходит
 *                за ними в базу
 *   object_ids   guid той же строки. Дублирует object_data, но обе
 *                части попадают в payload функции порознь
 *                (function.go: data["object_ids"], data["object_data"]),
 *                и написанные функции читают то одну, то другую
 *
 * Ручка v1: v2 принимает только вызов по пути функции
 * (`/v2/invoke_function/{path}`), а у кнопки сохранён id.
 *
 * Строки перезапрашиваются после успеха: функция для того и зовётся,
 * чтобы что-то поменять в строке, — а без этого правка появляется
 * только после перезагрузки страницы. Старая админка ровно этот вызов
 * и закомментировала, оставив кнопку, после которой «ничего не
 * произошло».
 */
export function useInvokeFunction(tableSlug: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ functionId, row }: { functionId: string; row: Item }) =>
      api.post<unknown>("/v1/invoke_function", {
        function_id: functionId,
        table_slug: tableSlug,
        object_data: row,
        object_ids: typeof row.guid === "string" ? [row.guid] : [],
      }),

    onError: (error) => reportError(error, "button.failed"),
    onSuccess: () => {
      toast.success(i18n.t("button.done"));
      void queryClient.invalidateQueries({ queryKey: keys.items.table(tableSlug) });
    },
  });
}
