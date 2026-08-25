import { Suspense, lazy } from "react";
import { IconCalendar, IconX } from "@tabler/icons-react";
import { Icon } from "./icon";
import { Popover } from "./popover";

/*
 * Календарь грузится отдельным куском и только когда открыли дату:
 * react-day-picker с локалями — это 100 КБ, и в общий бандл он попадать
 * не должен. Та же ленивая загрузка, что у редактора ячейки.
 */
const Calendar = lazy(() =>
  import("./calendar").then((module) => ({ default: module.Calendar })),
);

/**
 * Поле даты: кнопка с датой, под ней — наш календарь.
 *
 * Не `<input type="date">`: у нативного поля свой вид в каждой системе,
 * своя раскладка кнопок и светлый календарь в тёмной теме — рядом
 * с нашими кнопками он выглядит чужим. Внутри тот же `Calendar`, что
 * в редакторе ячейки, и тот же `Popover`, что у остальных списков.
 *
 * Значение — «ГГГГ-ММ-ДД», а не `Date`: в этом виде дату отдают и ждут
 * ручки, и лишнее преобразование на каждой стороне только добавляет
 * поводов ошибиться на часовом поясе.
 */
export function DatePicker({
  value,
  locale,
  placeholder = "",
  ariaLabel,
  className = "",
  clearLabel,
  onChange,
}: {
  /** «ГГГГ-ММ-ДД». Пусто — дата не выбрана. */
  value: string;
  /** Язык ИНТЕРФЕЙСА: им подписаны месяцы и им же форматируется дата. */
  locale: string;
  /** Что на кнопке, когда даты нет. */
  placeholder?: string;
  ariaLabel?: string;
  /** Ширина и высота задаются снаружи: у панели и у строки отбора свои. */
  className?: string;
  /** Подпись кнопки «очистить». Нет подписи — очистить нечем. */
  clearLabel?: string;
  onChange: (value: string) => void;
}) {
  // Строка без пояса читается как местная полночь, а не как UTC: иначе
  // календарь восточнее Гринвича подсвечивал бы вчерашний день.
  const day = value ? new Date(`${value}T00:00:00`) : undefined;
  const chosen = day && !Number.isNaN(day.getTime()) ? day : undefined;

  return (
    <Popover
      trigger={({ open, toggle }) => (
        <button
          type="button"
          onClick={toggle}
          aria-expanded={open}
          aria-haspopup="dialog"
          {...(ariaLabel ? { "aria-label": ariaLabel } : {})}
          className={`flex h-(--spacing-input) w-full items-center gap-2 rounded-md border bg-surface px-2.5 text-left text-sm transition-colors ${
            open ? "border-accent" : "border-border-strong hover:border-fg-subtle"
          } ${className}`}
        >
          <Icon as={IconCalendar} size={14} className="shrink-0 text-fg-subtle" />
          <span className={`flex-1 truncate ${chosen ? "text-fg" : "text-fg-subtle"}`}>
            {chosen ? chosen.toLocaleDateString(locale) : placeholder}
          </span>
        </button>
      )}
    >
      {(close) => (
        <Suspense fallback={<div className="h-64 w-64" />}>
          <Calendar
            locale={locale}
            value={chosen}
            onSelect={(date) => {
              onChange(toDayInput(date));
              // Выбор дня — законченное действие: выбирать больше нечего.
              close();
            }}
          />

          {clearLabel && chosen && (
            <button
              type="button"
              onClick={() => {
                onChange("");
                close();
              }}
              className="flex h-7 w-full items-center gap-1.5 rounded-md px-2 text-sm text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
            >
              <Icon as={IconX} size={14} />
              {clearLabel}
            </button>
          )}
        </Suspense>
      )}
    </Popover>
  );
}

/**
 * Дата из календаря → «ГГГГ-ММ-ДД». Части местные, а не UTC:
 * `toISOString()` у пользователя восточнее Гринвича вернул бы вчера.
 */
function toDayInput(date: Date): string {
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}
