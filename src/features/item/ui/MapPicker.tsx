import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import i18n from "@/shared/lib/i18n";
import {
  loadYandexMaps,
  type YandexMap,
  type YandexPlacemark,
} from "@/shared/lib/yandex-maps";
import type { Coords } from "../model/coords";

/**
 * Выбор точки кликом по карте — как в старой админке (HFMapField).
 *
 * Карта — способ УКАЗАТЬ координаты, а не их хранилище: наружу уходит
 * та же пара «широта, долгота», что и из полей ввода рядом. Поэтому
 * компонент не знает ни о поле, ни о ячейке — только точка и клик.
 *
 * ponytail: геолокации из старого кода нет — она спрашивала разрешение
 * браузера ради стартового центра карты. Центр без точки — Ташкент,
 * как в подсказках полей ввода. Вернуть легко, если попросят.
 */
const FALLBACK: [number, number] = [41.311081, 69.240562];
/** Город при пустой ячейке, улица — когда точка уже есть. */
const CITY_ZOOM = 11;
const STREET_ZOOM = 15;

export function MapPicker({
  point,
  onPick,
}: {
  point: Coords | null;
  onPick: (point: Coords) => void;
}) {
  const { t } = useTranslation();
  const box = useRef<HTMLDivElement>(null);
  const map = useRef<YandexMap | null>(null);
  const mark = useRef<YandexPlacemark | null>(null);
  const [failed, setFailed] = useState(false);

  /* Клик отдаёт наружу свежий обработчик, а карта создаётся один раз. */
  const pick = useRef(onPick);
  pick.current = onPick;

  const start = useRef(point);

  useEffect(() => {
    let disposed = false;

    loadYandexMaps(i18n.language)
      .then((api) => {
        if (disposed || !box.current) return;

        const at = start.current;
        const center: [number, number] = at ? [at.lat, at.lon] : FALLBACK;

        const created = new api.Map(box.current, {
          center,
          zoom: at ? STREET_ZOOM : CITY_ZOOM,
          controls: ["zoomControl"],
        });

        const placemark = new api.Placemark(center);
        created.geoObjects.add(placemark);
        created.events.add("click", (event) => {
          const [lat, lon] = event.get("coords");
          if (lat === undefined || lon === undefined) return;

          placemark.geometry.setCoordinates([lat, lon]);
          // Шесть знаков — метр точности; полный float в ячейке — шум.
          pick.current({ lat: Number(lat.toFixed(6)), lon: Number(lon.toFixed(6)) });
        });

        map.current = created;
        mark.current = placemark;
      })
      .catch(() => {
        if (!disposed) setFailed(true);
      });

    return () => {
      disposed = true;
      map.current?.destroy();
      map.current = null;
    };
  }, []);

  /* Точка, набранная руками в полях ввода, двигает метку и центр. */
  useEffect(() => {
    if (!point || !map.current || !mark.current) return;

    mark.current.geometry.setCoordinates([point.lat, point.lon]);
    map.current.setCenter([point.lat, point.lon]);
  }, [point?.lat, point?.lon]);

  if (failed) {
    return (
      <p className="grid h-24 place-items-center rounded-md border border-border text-2xs text-fg-subtle">
        {t("cell.mapUnavailable")}
      </p>
    );
  }

  return <div ref={box} className="h-52 w-full overflow-hidden rounded-md border border-border" />;
}
