import type { ReactNode } from "react";
import { IconX } from "@tabler/icons-react";

/**
 * Чип для значений Status и Multiselect. Пользователь выбирает оттенок
 * из фиксированного набора, а не произвольный HEX — иначе он подберёт
 * цвет под светлую тему, и в тёмной текст станет нечитаемым.
 */
export const CHIP_COLORS = [
  "gray",
  "blue",
  "green",
  "yellow",
  "orange",
  "red",
  "purple",
  "pink",
  "brown",
] as const;

export type ChipColor = (typeof CHIP_COLORS)[number];

/*
 * Классы перечислены целиком: Tailwind не видит собранные строкой имена.
 *
 * Наружу — ради событий календаря: там тот же оттенок ложится не на чип,
 * а на карточку целиком, и своя палитра рядом с этой разошлась бы
 * с первым же новым цветом.
 */
export const CHIP_STYLES: Record<ChipColor, string> = {
  gray: "bg-chip-gray-bg text-chip-gray-fg",
  blue: "bg-chip-blue-bg text-chip-blue-fg",
  green: "bg-chip-green-bg text-chip-green-fg",
  yellow: "bg-chip-yellow-bg text-chip-yellow-fg",
  orange: "bg-chip-orange-bg text-chip-orange-fg",
  red: "bg-chip-red-bg text-chip-red-fg",
  purple: "bg-chip-purple-bg text-chip-purple-fg",
  pink: "bg-chip-pink-bg text-chip-pink-fg",
  brown: "bg-chip-brown-bg text-chip-brown-fg",
};

/**
 * Тот же оттенок подложкой — для поверхности, а не для чипа: колонка
 * доски красится цветом своего варианта, и цвет должен быть заметен,
 * но не спорить с карточками поверх него.
 *
 * Прозрачность, а не отдельные токены: подложка ложится на фон
 * приложения, и в тёмной теме та же четверть даёт такой же приглушённый
 * оттенок, что и в светлой. Классы перечислены целиком — Tailwind
 * не видит собранные строкой имена.
 */
export const CHIP_SURFACE: Record<ChipColor, string> = {
  gray: "bg-chip-gray-bg/40",
  blue: "bg-chip-blue-bg/40",
  green: "bg-chip-green-bg/40",
  yellow: "bg-chip-yellow-bg/40",
  orange: "bg-chip-orange-bg/40",
  red: "bg-chip-red-bg/40",
  purple: "bg-chip-purple-bg/40",
  pink: "bg-chip-pink-bg/40",
  brown: "bg-chip-brown-bg/40",
};

/**
 * Оттенок → HEX для записи в данные.
 *
 * Обратная сторона hexToChipColor: цвет варианта хранится в базе как
 * HEX, и его читает не только эта админка — старый фронт, экспорт,
 * мобильные клиенты. Поэтому наружу уходит цвет, а не имя оттенка.
 *
 * Значения подобраны так, что hexToChipColor возвращает то же имя:
 * поле, созданное здесь, читается здесь же без сдвига оттенка.
 * Проверено тестом — на глаз это не проверяется.
 */
export const CHIP_HEX: Record<ChipColor, string> = {
  gray: "#9E9E9E",
  blue: "#3B82F6",
  green: "#22A06B",
  yellow: "#E3C11F",
  orange: "#F0842B",
  red: "#E5484D",
  purple: "#8B5CF6",
  pink: "#D6409F",
  brown: "#8B5E34",
};

/**
 * HEX из данных → оттенок палитры.
 *
 * Цвет варианта пользователь когда-то выбрал под светлую тему, и в базе
 * лежит именно HEX. Рисовать его как есть нельзя: #181D21 на тёмном фоне
 * это чёрный текст на чёрном. Поэтому цвет из данных читается как
 * НАМЕРЕНИЕ («красный», «зелёный»), а рисуется парой токенов, у которой
 * контраст проверен в обеих темах.
 *
 * Последняя версия конструктора уже даёт выбирать из четырёх именованных
 * цветов, а не пипеткой, — направление то же самое. Разбор нужен ради
 * данных, накопленных прежними версиями.
 */
export function hexToChipColor(hex: string): ChipColor {
  const rgb = parseHex(hex);
  if (!rgb) return "gray";

  const { hue, chroma, lightness } = toHsl(rgb);

  /*
   * Серым считаем всё, у чего цвета почти нет. Мера — хрома (разброс
   * каналов), а не насыщенность по HSL: та делится на близкую к нулю
   * величину у очень светлых и очень тёмных цветов и раздувается.
   * У #EAECF0 (светло-серый из палитры конструктора) насыщенность
   * по HSL выходит 0.17 и он становится «синим».
   */
  if (chroma < 0.1 || lightness < 0.12 || lightness > 0.97) return "gray";

  if (hue < 15 || hue >= 345) return "red";
  if (hue < 45) return lightness < 0.4 ? "brown" : "orange";
  if (hue < 70) return "yellow";
  if (hue < 165) return "green";
  if (hue < 255) return "blue";
  if (hue < 290) return "purple";
  return "pink";
}

type Rgb = { r: number; g: number; b: number };

function parseHex(hex: string): Rgb | null {
  const value = hex.trim().replace("#", "");
  // Допускаем и #abc, и #aabbcc, и #aabbccff — в данных встречается всё.
  const full =
    value.length === 3
      ? value
          .split("")
          .map((c) => c + c)
          .join("")
      : value.slice(0, 6);

  if (!/^[0-9a-f]{6}$/i.test(full)) return null;

  return {
    r: parseInt(full.slice(0, 2), 16) / 255,
    g: parseInt(full.slice(2, 4), 16) / 255,
    b: parseInt(full.slice(4, 6), 16) / 255,
  };
}

function toHsl({ r, g, b }: Rgb) {
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const delta = max - min;
  const lightness = (max + min) / 2;

  if (delta === 0) return { hue: 0, chroma: 0, lightness };

  let hue: number;
  if (max === r) hue = ((g - b) / delta) % 6;
  else if (max === g) hue = (b - r) / delta + 2;
  else hue = (r - g) / delta + 4;

  hue *= 60;
  if (hue < 0) hue += 360;

  return { hue, chroma: delta, lightness };
}

export function Chip({
  color = "gray",
  dot,
  onRemove,
  removeLabel,
  children,
}: {
  color?: ChipColor | undefined;
  /** Точка слева — признак Status: у него значение всегда одно. */
  dot?: boolean | undefined;
  /** Крестик справа. Без обработчика не рисуется. */
  onRemove?: (() => void) | undefined;
  removeLabel?: string | undefined;
  children: ReactNode;
}) {
  return (
    /*
     * min-w-0 — не украшение: у элемента flex минимальная ширина равна
     * содержимому, и ряд из двух чипов не сжимался, а вылезал за край
     * (на карточке доски — прямо поверх соседней колонки). max-w-full
     * этого не ловит: он ограничивает КАЖДЫЙ чип шириной ряда, а их
     * сумму — нет. С min-w-0 длинные чипы ужимаются вместе и обрезаются
     * многоточием каждый.
     */
    <span
      className={`inline-flex h-5 max-w-full min-w-0 items-center gap-1 rounded-sm px-1.5 text-xs ${CHIP_STYLES[color]}`}
    >
      {/* Цвет точки — сам текст чипа: третий токен на каждый оттенок
          пришлось бы держать в двух темах ради четырёх пикселей. */}
      {dot && <span aria-hidden className="size-1.5 shrink-0 rounded-full bg-current opacity-70" />}
      <span className="truncate">{children}</span>

      {onRemove && (
        <button
          type="button"
          onClick={onRemove}
          aria-label={removeLabel}
          className="-mr-0.5 grid size-3.5 shrink-0 place-items-center rounded-sm opacity-60 transition-opacity hover:opacity-100"
        >
          <IconX size={10} stroke={2.5} aria-hidden />
        </button>
      )}
    </span>
  );
}
