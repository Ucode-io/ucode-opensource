import { useEffect, useState, type ReactNode } from "react";

/**
 * Иконка, имя которой задаёт бэкенд. Три источника в одном поле —
 * так исторически устроено в ucode:
 *
 *   "mdi:home"           → Iconify
 *   "https://…/x.svg"    → произвольный URL
 *   "folder-new.svg"     → файл в нашем CDN
 *
 * Файлы из CDN и Iconify вставляются инлайном, чтобы иконка красилась
 * currentColor и работала в тёмной теме. Произвольный URL вставляется
 * через <img>: чужой origin инлайнить нельзя — это чужая разметка
 * внутри нашей страницы.
 *
 * Толщина линий приводится к нашей классом .icon-normalized — иначе
 * чужие иконки стоят в одном ряду с Tabler и выглядят чужими.
 * Заливные иконки так не исправить: у них нет обводки, и остаётся
 * только заменить их в настройках пункта.
 */

const CDN = import.meta.env.VITE_ICON_CDN_URL;
const ICONIFY = "https://api.iconify.design";

/** Один и тот же файл на весь сеанс скачивается один раз. */
const cache = new Map<string, Promise<string>>();

function load(url: string): Promise<string> {
  const cached = cache.get(url);
  if (cached) return cached;

  const promise = fetch(url)
    .then((response) => (response.ok ? response.text() : ""))
    // Принимаем только svg: ошибка CDN не должна попасть в разметку.
    .then((text) => (text.trimStart().startsWith("<svg") ? forceCurrentColor(text) : ""))
    .catch(() => "");

  cache.set(url, promise);
  return promise;
}

/** Иконки приходят с зашитым цветом — снимаем его, чтобы работала тема. */
function forceCurrentColor(svg: string): string {
  return svg
    .replace(/fill="(?!none)[^"]*"/g, 'fill="currentColor"')
    .replace(/stroke="(?!none)[^"]*"/g, 'stroke="currentColor"');
}

type Source = { kind: "inline"; url: string } | { kind: "img"; url: string } | null;

export function resolveIconSource(name: string): Source {
  if (!name) return null;
  if (name.startsWith("http://") || name.startsWith("https://")) return { kind: "img", url: name };

  if (name.includes(":")) {
    const [prefix, ...rest] = name.split(":");
    const icon = rest.join(":");
    if (!prefix || !icon) return null;
    return { kind: "inline", url: `${ICONIFY}/${prefix}/${icon}.svg` };
  }

  return { kind: "inline", url: `${CDN}${name}` };
}

export function DynamicIcon({
  name,
  size = 16,
  fallback,
}: {
  name: string;
  size?: number;
  fallback: ReactNode;
}) {
  const source = resolveIconSource(name);
  const [svg, setSvg] = useState("");

  useEffect(() => {
    if (source?.kind !== "inline") return;

    let alive = true;
    void load(source.url).then((text) => alive && setSvg(text));
    return () => {
      alive = false;
    };
  }, [source?.kind, source?.url]);

  if (!source) return <>{fallback}</>;

  if (source.kind === "img") {
    return <img src={source.url} width={size} height={size} alt="" className="shrink-0" />;
  }

  // Пока файл не пришёл или не пришёл вовсе — своя иконка по типу пункта.
  if (!svg) return <>{fallback}</>;

  return (
    <span
      aria-hidden
      className="icon-normalized inline-flex shrink-0 items-center justify-center [&>svg]:size-full"
      style={{ width: size, height: size }}
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
