import { useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import i18n from "@/shared/lib/i18n";
import { loadYandexMaps, type YandexMap, type YandexPolygon } from "@/shared/lib/yandex-maps";
import type { Coords } from "../model/coords";
import { polygonCenter } from "../model/polygon";

/**
 * Область обводится мышью по карте — как в старой админке
 * (`HFPolygonFieldCellEditor`, те же Яндекс.Карты и тот же ключ).
 *
 * Карта — способ УКАЗАТЬ координаты, а не их хранилище: наружу уходит
 * тот же список точек, что лежит в колонке. Поэтому компонент не знает
 * ни о поле, ни о ячейке — только точки и обводка.
 *
 * ponytail: одна область, без дырок и без нескольких контуров. Данные
 * их допускают (`model/polygon` внутренние контуры отбрасывает), но
 * рисовать их старая не давала тоже, а редактор нескольких контуров —
 * это уже ГИС, а не поле в таблице.
 */
const FALLBACK: [number, number] = [41.311081, 69.240562];
const ZOOM = 11;

export function PolygonPicker({
  points,
  onDraw,
  fallback,
}: {
  points: Coords[];
  onDraw: (points: number[][]) => void;
  /** Что показать, если карта не поднялась: форма области из координат. */
  fallback: ReactNode;
}) {
  const box = useRef<HTMLDivElement>(null);
  const map = useRef<YandexMap | null>(null);
  const area = useRef<YandexPolygon | null>(null);
  const [failed, setFailed] = useState(false);

  /* Обводка отдаёт наружу свежий обработчик, а карта создаётся один раз. */
  const draw = useRef(onDraw);
  draw.current = onDraw;

  const start = useRef(points);

  /*
   * Карта шлёт «change» и на обводку мышью, и на нашу подстановку
   * координат — причём синхронно, прямо внутри setCoordinates
   * (проверено на живом api-maps 2.1). Флаг отличает одно от другого:
   * без него правка текста двигала бы карту, карта — текст, и так
   * по кругу. Сравнением значений тут не обойтись: карта ЗАМЫКАЕТ
   * контур, дописывая первую точку в конец, и отданное ей никогда
   * не равно тому, что она вернёт.
   */
  const applying = useRef(false);

  useEffect(() => {
    let disposed = false;

    loadYandexMaps(i18n.language)
      .then((api) => {
        if (disposed || !box.current) return;

        const ring = toRing(start.current);
        const center = polygonCenter(start.current);

        const created = new api.Map(box.current, {
          center: center ? [center.lat, center.lon] : FALLBACK,
          zoom: ZOOM,
          controls: ["zoomControl"],
        });

        /*
         * Цвет карта рисует своим канвасом и о переменных css не знает,
         * поэтому токен читается значением. Не захардкоженный цвет:
         * тот же `--color-accent-solid`, что у кнопок, — на светлых
         * тайлах он контрастнее `--color-accent`. Пустое значение
         * (токена нет) отдаём как есть: тогда карта возьмёт свой цвет.
         */
        const accent = getComputedStyle(box.current)
          .getPropertyValue("--color-accent-solid")
          .trim();

        const polygon = new api.Polygon(
          [ring],
          {},
          {
            editorDrawingCursor: "crosshair",
            fillColor: accent,
            fillOpacity: 0.25,
            strokeColor: accent,
            strokeWidth: 2,
          },
        );

        created.geoObjects.add(polygon);

        polygon.geometry.events.add("change", () => {
          if (applying.current) return;

          draw.current(polygon.geometry.getCoordinates()?.[0] ?? []);
        });

        // Пустая область — ставим точки кликами, у заполненной двигаем
        // уже поставленные.
        if (ring.length) polygon.editor.startEditing();
        else polygon.editor.startDrawing();

        map.current = created;
        area.current = polygon;
      })
      .catch(() => {
        if (!disposed) setFailed(true);
      });

    return () => {
      disposed = true;
      map.current?.destroy();
      map.current = null;
      area.current = null;
    };
  }, []);

  /*
   * Координаты, набранные руками в тексте, перерисовывают область.
   *
   * Пустой список не применяется, и это не мелочь: пока строку
   * дописывают, JSON в ней разбираться перестаёт — «[[[41.3,» это
   * ноль точек. Стирать по нему нарисованное значило бы гасить область
   * на каждом промежуточном символе. Пустое значение всё равно уедет
   * в колонку: сохраняется текст, а не то, что нарисовано.
   */
  useEffect(() => {
    const polygon = area.current;
    if (!polygon) return;

    const ring = toRing(points);
    if (!ring.length) return;

    // Уже нарисовано ровно это (карта сама вернула нам свой контур) —
    // перерисовывать нечего.
    const current = polygon.geometry.getCoordinates()?.[0] ?? [];
    if (JSON.stringify(current) === JSON.stringify(ring)) return;

    applying.current = true;
    polygon.geometry.setCoordinates([ring]);
    applying.current = false;
  }, [points]);

  if (failed) return <>{fallback}</>;

  return <div ref={box} className="h-52 w-full overflow-hidden rounded-md border border-border" />;
}

/** Точки → контур в том виде, в каком их держит карта. */
function toRing(points: Coords[]): number[][] {
  return points.map((point) => [point.lat, point.lon]);
}
