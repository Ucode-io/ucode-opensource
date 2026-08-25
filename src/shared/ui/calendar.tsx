import { useEffect, useRef, type ChangeEvent } from "react";
import { IconChevronLeft, IconChevronRight } from "@tabler/icons-react";
import { DayPicker } from "react-day-picker";
import { ru, uz } from "react-day-picker/locale";
import { Dropdown } from "./dropdown";
import { Icon } from "./icon";

/**
 * Календарь. Обёртка над react-day-picker — тем же, на котором стоит
 * календарь shadcn.
 *
 * Своя разметка, а не их стили: библиотека даёт сетку, недели, месяцы,
 * клавиатуру и локали (это месяц работы), а цвета и размеры у нас свои
 * и обязаны жить в токенах. Поэтому style.css не подключается вовсе —
 * каждый элемент получает класс через classNames.
 *
 * Локаль дат приходит снаружи: это язык ИНТЕРФЕЙСА (ru/en/uz), а не
 * язык данных. Английский — по умолчанию у самой библиотеки.
 */

const LOCALES = { ru, uz };

/* Классы вынесены: иначе строка classNames становится нечитаемой. */
const dayButton =
  "grid size-8 place-items-center rounded-md text-sm transition-colors hover:bg-surface-hover";

const navButton =
  "grid size-7 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg disabled:pointer-events-none disabled:opacity-30";

/*
 * Границы списка годов. Без них выбор месяца и года сводится к одному
 * текущему году, а дата рождения из пустой ячейки набиралась стрелкой
 * «предыдущий месяц» — четыреста щелчков.
 *
 * Сто назад — это дата рождения, десять вперёд — срок и план. Уже
 * записанное значение раздвигает границы: дата из архива не должна
 * стать недостижимой.
 */
const YEARS_BACK = 100;
const YEARS_AHEAD = 10;

export function Calendar({
  value,
  onSelect,
  locale,
}: {
  value: Date | undefined;
  onSelect: (date: Date) => void;
  /** Язык интерфейса: ru, en, uz. */
  locale: string;
}) {
  const now = new Date().getFullYear();
  const year = value?.getFullYear() ?? now;

  return (
    <DayPicker
      mode="single"
      selected={value}
      // Открываемся на месяце значения, а не на текущем: иначе правка
      // прошлогодней даты начинается с прокрутки на двенадцать месяцев.
      {...(value ? { defaultMonth: value } : {})}
      captionLayout="dropdown"
      startMonth={new Date(Math.min(now - YEARS_BACK, year), 0)}
      endMonth={new Date(Math.max(now + YEARS_AHEAD, year), 11)}
      onSelect={(date) => date && onSelect(date)}
      locale={LOCALES[locale as keyof typeof LOCALES]}
      showOutsideDays
      /* Неделя с понедельника во всех трёх языках: у enUS она с воскресенья,
         а таблица одна и та же для всех пользователей проекта. */
      weekStartsOn={1}
      components={{
        Chevron: ({ orientation }) => (
          <Icon as={orientation === "left" ? IconChevronLeft : IconChevronRight} size={16} />
        ),
        /*
         * Месяц и год — нашим списком, а не родным `<select>`, который
         * рисует библиотека. Системное меню не знает тёмной темы
         * и в каждой ОС выглядит по-своему, а стоит оно посреди нашего
         * календаря.
         *
         * Событие подделываем: обработчик библиотеки читает из него
         * ровно `target.value` и больше ничего (DayPicker.js,
         * handleMonthChange).
         */
        Dropdown: ({ options = [], value, onChange, disabled, className, ...rest }) => (
          <Dropdown
            size="sm"
            className={className ?? ""}
            value={String(value ?? "")}
            {...(rest["aria-label"] ? { ariaLabel: rest["aria-label"] } : {})}
            disabled={disabled ?? false}
            items={options.map((option) => ({
              value: String(option.value),
              label: option.label,
            }))}
            onChange={(next) =>
              onChange?.({ target: { value: next } } as ChangeEvent<HTMLSelectElement>)
            }
          />
        ),
      }}
      classNames={{
        root: "relative w-fit select-none",
        months: "flex flex-col",
        month: "space-y-1",
        month_caption: "flex h-8 items-center px-1 text-sm font-medium",
        caption_label: "flex items-center gap-0.5 capitalize",
        dropdowns: "flex items-center gap-1",
        /* Ширины фиксированные: подпись месяца меняется от «май»
           до «сентябрь», и без них соседний список ездил бы вбок
           при каждом перелистывании. */
        months_dropdown: "w-32",
        years_dropdown: "w-20",
        nav: "absolute right-0 top-0 flex items-center gap-0.5",
        button_previous: navButton,
        button_next: navButton,
        month_grid: "w-full border-collapse",
        weekdays: "flex",
        weekday: "grid size-8 place-items-center text-2xs font-normal text-fg-muted capitalize",
        week: "flex",
        day: "p-0",
        day_button: dayButton,
        /* Состояния навешиваются на ячейку, а красится кнопка внутри —
           у неё фон и скругление. Поэтому вложенный селектор. */
        selected: "[&>button]:bg-accent-solid [&>button]:text-accent-fg",
        today: "[&>button]:font-semibold [&>button]:text-accent-text",
        outside: "[&>button]:text-fg-subtle",
        disabled: "[&>button]:pointer-events-none [&>button]:opacity-30",
        hidden: "invisible",
      }}
    />
  );
}

/**
 * Часы и минуты двумя прокручиваемыми столбцами.
 *
 * Не <input type="time">: у браузерного поля свой формат, свои стрелки
 * и своя раскладка в каждой системе, и рядом с нашим календарём он
 * выглядит чужим. Список — это два .map и прокрутка к выбранному.
 */
export function TimeList({
  value,
  onChange,
}: {
  /** «ЧЧ:ММ». Пусто — ничего не выбрано. */
  value: string;
  onChange: (value: string) => void;
}) {
  const [hour = "", minute = ""] = value.split(":");

  return (
    <div className="flex h-56 gap-1 border-l border-border pl-1">
      <TimeColumn
        values={UNITS.hours}
        selected={hour}
        // Час без минут — это не время: доставляем нулями, а не молчим.
        onSelect={(next) => onChange(`${next}:${minute || "00"}`)}
      />
      <TimeColumn
        values={UNITS.minutes}
        selected={minute}
        onSelect={(next) => onChange(`${hour || "00"}:${next}`)}
      />
    </div>
  );
}

const UNITS = {
  hours: range(24),
  minutes: range(60),
};

function TimeColumn({
  values,
  selected,
  onSelect,
}: {
  values: string[];
  selected: string;
  onSelect: (value: string) => void;
}) {
  const list = useRef<HTMLDivElement>(null);
  const active = useRef<HTMLButtonElement>(null);

  /*
   * Прокрутка к выбранному — присваиванием scrollTop, а не
   * scrollIntoView: тот прокручивает и родителей, а редактор ячейки
   * закрывается по любой прокрутке снаружи себя.
   *
   * Контейнер позиционирован (relative) не ради вида: offsetTop
   * отсчитывается от ближайшего позиционированного предка.
   */
  useEffect(() => {
    const container = list.current;
    const button = active.current;
    if (!container || !button) return;

    container.scrollTop = button.offsetTop - container.clientHeight / 2 + button.clientHeight / 2;
  }, []);

  return (
    <div ref={list} className="relative w-11 overflow-y-auto">
      {values.map((item) => {
        const isActive = item === selected;

        return (
          <button
            key={item}
            ref={isActive ? active : undefined}
            type="button"
            onClick={() => onSelect(item)}
            className={`block w-full rounded-md py-1 text-center text-sm tabular-nums transition-colors ${
              isActive ? "bg-accent-solid text-accent-fg" : "hover:bg-surface-hover"
            }`}
          >
            {item}
          </button>
        );
      })}
    </div>
  );
}

function range(count: number): string[] {
  return Array.from({ length: count }, (_, index) => String(index).padStart(2, "0"));
}
