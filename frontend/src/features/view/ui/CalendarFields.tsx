import { IconX } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { localized, type Field } from "@/features/table";
import { fieldIcon } from "@/features/item";
import { Icon } from "@/shared/ui/icon";
import { PopoverItem } from "@/shared/ui/popover";
import type { View } from "../model/types";
import { viewIcon } from "./view-icon";

/**
 * Выбор полей дат календаря: с какого поля событие начинается и каким
 * заканчивается.
 *
 * Один компонент на два места — страницу настроек view и пустой экран
 * календаря. Это не украшение: без поля начала календарь не рисует
 * ничего, и настройка, спрятанная за «⋯», превращает пустой экран
 * в тупик. Тот же выбор в старой админке лежал в панели фильтров
 * (HeaderFilter/CalendarSettings: «Time from» и «Time to»), и из пустого
 * календаря к нему тоже приходилось догадываться.
 *
 * Правки уходят по одной и сразу, без кнопки «сохранить»: так же ведут
 * себя остальные настройки view.
 */

/**
 * Типы полей, которые годятся событию. Ровно те, в которых лежит дата:
 * время без дня (TIME) на сетку не посадить — неизвестно, в какой день.
 *
 * Старая админка предлагала здесь ВСЕ колонки таблицы, включая строки
 * и числа (CalendarSettings.jsx: `listToLanOptions(columns, …)`),
 * и выбранная не та колонка давала пустой календарь без объяснений.
 */
const DATE_TYPES = new Set(["DATE", "DATE_TIME", "DATE_TIME_WITHOUT_TIME_ZONE"]);

export function dateFields(fields: Field[]): Field[] {
  return fields.filter((field) => DATE_TYPES.has(field.type));
}

export function CalendarFields({
  from,
  to,
  dates,
  language,
  onDateFrom,
  onDateTo,
}: {
  /** Выбранные слаги. Строками, а не view: тот же выбор делают
      и до создания view, когда его ещё нет. */
  from: string;
  to: string;
  /** Поля с датой. Отбор и поиск делает вызывающий. */
  dates: Field[];
  language: string;
  onDateFrom: (slug: string) => void;
  onDateTo: (slug: string) => void;
}) {
  const { t } = useTranslation();

  const list = (slug: string, onPick: (slug: string) => void, skip = "") =>
    dates
      /*
       * Одно и то же поле началом и концом — событие нулевой длины,
       * то есть настройка, которая ничего не значит. Старая админка
       * такой выбор разрешала и потом ругалась красным окном
       * («Date from and date to are same», TimeLineBlock.jsx), уже
       * ничего не рисуя. Проще не предлагать. Ранее выбранное поле
       * из списка не пропадает: иначе не видно, что там стоит.
       */
      .filter((field) => field.slug !== skip || field.slug === slug)
      .map((field) => (
        <PopoverItem
          key={field.id}
          active={slug === field.slug}
          icon={<Icon as={fieldIcon(field.type)} size={16} className="shrink-0" />}
          onClick={() => onPick(field.slug)}
        >
          {localized(field.labels, language, field.label)}
        </PopoverItem>
      ));

  return (
    <>
      <p className="px-2 pt-1 text-2xs text-fg-subtle">{t("view.calendarFrom")}</p>
      {list(from, onDateFrom, to)}

      <div className="my-1 h-px bg-border" />

      <p className="px-2 pt-1 text-2xs text-fg-subtle">{t("view.calendarTo")}</p>
      {/* «Без конца» — не пустая строка списка, а осмысленный выбор:
          событие тогда точка в дне, а не полоса на сутки. */}
      <PopoverItem
        active={!to}
        icon={<Icon as={IconX} size={16} className="shrink-0 text-fg-subtle" />}
        onClick={() => onDateTo("")}
      >
        {t("view.calendarToNone")}
      </PopoverItem>
      {list(to, onDateTo, from)}
    </>
  );
}

/**
 * Пустой календарь: объяснение и тот же выбор полей на месте.
 *
 * Без права на настройку — только объяснение: роль, которой не дают
 * править view, не должна видеть выбор, который ответит 403.
 */
export function CalendarSetup({
  view,
  fields,
  language,
  onDateFrom,
  onDateTo,
}: {
  view: View;
  /** ВСЕ поля таблицы: срок бывает и не показан колонкой. */
  fields: Field[];
  language: string;
  /** Нет обработчиков — экран только объясняет, но не предлагает выбор. */
  onDateFrom?: ((slug: string) => void) | undefined;
  onDateTo?: ((slug: string) => void) | undefined;
}) {
  const { t } = useTranslation();
  const dates = dateFields(fields);
  /* Настройка одна на два экрана, а слова разные: на таймлайне
     «календарь не настроен» — это про чужой экран. */
  const words = view.type === "TIMELINE" ? "timeline" : "calendar";

  return (
    <div className="flex flex-1 items-start justify-center overflow-y-auto p-8">
      <div className="flex w-full max-w-sm flex-col items-center gap-2 text-center">
        <span className="grid size-10 place-items-center rounded-lg border border-border text-fg-subtle">
          <Icon as={viewIcon(view.type)} size={20} />
        </span>

        <h2 className="text-sm font-medium">{t(`${words}.setupTitle` as const)}</h2>
        <p className="text-xs text-fg-muted">{t(`${words}.setupHint` as const)}</p>

        {onDateFrom && onDateTo && (
          <div className="mt-2 w-full rounded-lg border border-border bg-surface p-1 text-left">
            {dates.length ? (
              <CalendarFields
                from={view.dateFromSlug}
                to={view.dateToSlug}
                dates={dates}
                language={language}
                onDateFrom={onDateFrom}
                onDateTo={onDateTo}
              />
            ) : (
              /* Список пуст не «потому что не нашлось»: в таблице нет
                 ни одного поля с датой, и чинится это полем, а не
                 настройкой календаря. */
              <p className="px-2 py-1.5 text-2xs text-fg-subtle">{t("view.calendarEmpty")}</p>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
