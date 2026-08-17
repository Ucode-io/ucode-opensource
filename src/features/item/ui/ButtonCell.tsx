import { IconBolt, IconLoader2 } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import type { Field } from "@/features/table";
import { DynamicIcon } from "@/shared/ui/dynamic-icon";
import { Icon } from "@/shared/ui/icon";
import { useInvokeFunction } from "../api/functions";
import type { Item } from "../model/types";

/**
 * Поле-кнопка: значения у него нет, есть действие.
 *
 * В колонке базы всегда пусто — она заведена только затем, чтобы кнопке
 * было где стоять. Что нажали, знает бэкенд: клик зовёт функцию,
 * настроенную в поле (attributes.function), и передаёт ей строку целиком.
 *
 * Клик не всплывает: ячейка над ним открывает раскрытую карточку, и одно
 * нажатие делало бы два действия сразу.
 */
export function ButtonCell({
  field,
  row,
  tableSlug,
}: {
  field: Field;
  row: Item;
  tableSlug: string;
}) {
  const { t } = useTranslation();
  const invoke = useInvokeFunction(tableSlug);

  const functionId = textOf(field.attributes["function"]);
  const icon = textOf(field.attributes["icon"]);
  const label = field.label || t("button.run");

  return (
    <button
      type="button"
      // Функция не выбрана — кнопка есть, а звать нечего. Показываем её
      // выключенной, а не молчащей: иначе выглядит как сломанная.
      disabled={!functionId || invoke.isPending}
      title={functionId ? label : t("button.noFunction")}
      aria-label={label}
      onClick={(event) => {
        event.stopPropagation();
        invoke.mutate({ functionId, row });
      }}
      className="grid size-7 shrink-0 place-items-center rounded-md border border-border-strong text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg disabled:opacity-40"
    >
      {invoke.isPending ? (
        <Icon as={IconLoader2} size={14} className="animate-spin" />
      ) : (
        /* Иконку задаёт админ; не задал — молния, как у действия
           вообще. Своей подписи у кнопки в ячейке нет: колонка узкая,
           и подпись поля уже стоит в её заголовке. */
        <DynamicIcon name={icon} fallback={<Icon as={IconBolt} size={14} />} />
      )}
    </button>
  );
}

function textOf(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}
