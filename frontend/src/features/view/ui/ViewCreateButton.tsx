import { useState } from "react";
import { IconChevronLeft, IconPlus } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import type { Field } from "@/features/table";
import type { TranslationKey } from "@/shared/lib/i18n";
import { Icon } from "@/shared/ui/icon";
import { Input } from "@/shared/ui/input";
import { Popover, PopoverItem } from "@/shared/ui/popover";
import { IMPLEMENTED_VIEW_TYPES, VIEW_TYPES } from "../model/types";
import { CalendarFields, dateFields } from "./CalendarFields";
import { viewIcon } from "./view-icon";

/**
 * Новая вкладка: имя и список типов, как в старой админке, — выбор типа
 * и есть создание.
 *
 * Кроме календаря и таймлайна. У них выбор типа — ещё не всё: без поля,
 * с которого начинается событие, они не рисуют ничего, и созданный
 * «пустым» view открывается экраном настройки вместо сетки. Поэтому
 * у них есть второй шаг — тот же, что и в старой админке, где «Date
 * from» и «Date to» спрашивались прямо в окне создания
 * (useViewCreatePopupProps.jsx: «Please select date range»).
 *
 * Разница одна: обязательно только начало. Старая форма требовала обе
 * даты, но событие без конца — это точка в дне, законный случай,
 * и запрещать его незачем.
 *
 * У доски второго шага нет: поле раскладки выбирают в настройках уже
 * созданной. Это не забывчивость — доска без него показывает ту же
 * подсказку и ту же настройку, а лишний шаг платят все, включая тех,
 * кто заводит обычную таблицу.
 *
 * Имя необязательно: без него вкладка называется своим типом, ровно как
 * все view, созданные до появления имён.
 */
const CREATABLE = VIEW_TYPES.filter((type) => IMPLEMENTED_VIEW_TYPES.has(type));

/** Типы, которым нужны поля дат. Без начала им нечего показывать. */
const DATED = new Set<string>(["CALENDAR", "TIMELINE"]);

/** Даты нового календаря. Пусто у остальных типов. */
export type NewViewDates = { dateFrom: string; dateTo: string };

export function ViewCreateButton({
  busy,
  fields,
  language,
  onCreate,
}: {
  busy: boolean;
  /** Поля таблицы: из них выбираются даты календаря. */
  fields: Field[];
  /** Язык ДАННЫХ: подписи полей хранятся на языках проекта. */
  language: string;
  onCreate: (name: string, type: string, dates?: NewViewDates) => void;
}) {
  const { t } = useTranslation();

  return (
    <Popover
      trigger={({ open, toggle }) => (
        <button
          type="button"
          onClick={toggle}
          disabled={busy}
          aria-expanded={open}
          aria-label={t("view.create")}
          title={t("view.create")}
          className="grid size-7 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-fg disabled:opacity-50"
        >
          <Icon as={IconPlus} size={16} />
        </button>
      )}
    >
      {(close) => (
        <CreateForm
          fields={fields}
          language={language}
          onSubmit={(name, type, dates) => {
            onCreate(name, type, dates);
            close();
          }}
        />
      )}
    </Popover>
  );
}

function CreateForm({
  fields,
  language,
  onSubmit,
}: {
  fields: Field[];
  language: string;
  onSubmit: (name: string, type: string, dates?: NewViewDates) => void;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  /**
   * Второй шаг: выбранные поля дат и тип, ради которого их спрашивают.
   * null — шага нет.
   */
  const [dates, setDates] = useState<(NewViewDates & { type: string }) | null>(null);

  const dateOptions = dateFields(fields);

  const pick = (type: string) => {
    /*
     * Без единого поля с датой второго шага нет: выбирать не из чего.
     * View создаётся и открывается экраном настройки, который скажет
     * то же самое, но словами.
     */
    if (DATED.has(type) && dateOptions.length) {
      setDates({ type, dateFrom: "", dateTo: "" });
      return;
    }

    onSubmit(name.trim(), type);
  };

  if (dates) {
    return (
      <div className="flex w-64 flex-col gap-1 p-1">
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={() => setDates(null)}
            aria-label={t("action.back")}
            className="grid size-7 shrink-0 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
          >
            <Icon as={IconChevronLeft} size={16} />
          </button>
          <span className="truncate text-sm font-medium">{t("view.calendarFields")}</span>
        </div>

        <p className="px-2 text-2xs text-fg-subtle">{t("view.calendarFieldsHint")}</p>

        <div className="max-h-64 overflow-y-auto">
          <CalendarFields
            from={dates.dateFrom}
            to={dates.dateTo}
            dates={dateOptions}
            language={language}
            onDateFrom={(dateFrom) => setDates({ ...dates, dateFrom })}
            onDateTo={(dateTo) => setDates({ ...dates, dateTo })}
          />
        </div>

        {/* Кнопка, а не создание по выбору поля: полей два, и первое
            из них — ещё не готовая настройка. */}
        <button
          type="button"
          disabled={!dates.dateFrom}
          onClick={() => onSubmit(name.trim(), dates.type, dates)}
          className="h-8 shrink-0 rounded-md bg-accent-solid text-sm font-medium text-accent-fg transition-opacity hover:opacity-90 disabled:opacity-40"
        >
          {t("action.create")}
        </button>
      </div>
    );
  }

  return (
    <form
      className="flex w-56 flex-col gap-1 p-1"
      /* Enter в поле имени — самый частый путь: обычная таблица. */
      onSubmit={(event) => {
        event.preventDefault();
        pick(CREATABLE[0] ?? "TABLE");
      }}
    >
      <Input
        autoFocus
        value={name}
        onChange={(event) => setName(event.target.value)}
        placeholder={t("view.namePlaceholder")}
        aria-label={t("view.name")}
      />

      {CREATABLE.map((type) => (
        <PopoverItem
          key={type}
          icon={<Icon as={viewIcon(type)} size={16} className="shrink-0" />}
          onClick={() => pick(type)}
        >
          {t(`view.type.${type}` as TranslationKey, { defaultValue: type })}
        </PopoverItem>
      ))}
    </form>
  );
}
