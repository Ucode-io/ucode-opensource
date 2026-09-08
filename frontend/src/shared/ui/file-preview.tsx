import { useEffect } from "react";
import { create } from "zustand";
import { IconChevronLeft, IconChevronRight, IconDownload, IconFile, IconX } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { fileKind, fileName, type FileKind } from "@/shared/lib/file-kind";
import { Icon } from "@/shared/ui/icon";
import { Modal } from "@/shared/ui/modal";

/**
 * Просмотр файла поверх экрана: картинка, видео, звук, PDF.
 *
 * Один на всё приложение, а не по компоненту на каждое место с файлами.
 * Открывают его из ячейки таблицы, из карточки записи, из редактора
 * поля — а окно должно быть одно и вести себя одинаково: те же клавиши,
 * та же перелистовка, тот же способ скачать.
 *
 * Отсюда и store: место показа знает только «вот список файлов и вот
 * который открыли», а окно живёт в корне приложения и переживает
 * перерисовку того, из чего его открыли. Прокидывать через пропсы
 * пришлось бы через таблицу, строку, ячейку и редактор — четыре слоя
 * ради модалки.
 */
type PreviewState = {
  /** Файлы, между которыми листают. Пусто — окна нет. */
  files: string[];
  index: number;
  /**
   * Чем показывать, если по ссылке этого не понять. Ячейка знает тип
   * своего поля, а имя файла — нет: ucode дописывает при загрузке тип
   * поля вместо расширения, и встречаются файлы вовсе без него.
   */
  kind: FileKind | undefined;
  open: (files: string[], index?: number, kind?: FileKind) => void;
  close: () => void;
  step: (delta: number) => void;
};

export const useFilePreview = create<PreviewState>((set) => ({
  files: [],
  index: 0,
  kind: undefined,
  open: (files, index = 0, kind) => set({ files: files.filter(Boolean), index, kind }),
  close: () => set({ files: [], index: 0, kind: undefined }),
  /*
   * Перелистывание по кругу: на последнем файле «вперёд» ведёт
   * к первому. Так проще, чем гасить кнопки на краях, и ни один клик
   * не оказывается холостым.
   */
  step: (delta) =>
    set((state) => ({
      index: (state.index + delta + state.files.length) % state.files.length,
    })),
}));

/** Открыть просмотр, не подписываясь на его состояние (для обработчиков). */
export const openPreview = (files: string[], index = 0, kind?: FileKind) =>
  useFilePreview.getState().open(files, index, kind);

export function FilePreview() {
  const { t } = useTranslation();
  const { files, index, kind, close, step } = useFilePreview();
  const url = files[index];

  /*
   * Клавиатура: Esc закрывает, стрелки листают. Слушатель на документе,
   * а не на разметке окна: фокус после открытия остаётся там, где был,
   * и на самом окне его может не быть вовсе.
   */
  useEffect(() => {
    if (!url) return;

    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
      if (event.key === "ArrowLeft") step(-1);
      if (event.key === "ArrowRight") step(1);
    };

    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [url, close, step]);

  if (!url) return null;

  return (
    <Modal
      onClose={close}
      /* Просмотрщик занимает экран целиком: файл по центру, шапка сверху. */
      className="flex flex-col"
      background="var(--color-overlay-strong)"
    >
      <header
        className="flex h-12 shrink-0 items-center gap-2 px-3 text-sm text-white"
        onPointerDown={(event) => event.stopPropagation()}
      >
        <span className="min-w-0 flex-1 truncate">{fileName(url)}</span>

        {files.length > 1 && (
          <span className="shrink-0 tabular-nums opacity-70">
            {index + 1} / {files.length}
          </span>
        )}

        <a
          href={url}
          download
          target="_blank"
          rel="noreferrer noopener"
          aria-label={t("cell.download")}
          title={t("cell.download")}
          className={button}
        >
          <Icon as={IconDownload} size={18} />
        </a>

        <button type="button" onClick={close} aria-label={t("action.close")} className={button}>
          <Icon as={IconX} size={18} />
        </button>
      </header>

      {/* Полоса растянута по высоте (items-stretch по умолчанию):
          без этого у середины нет заданной высоты, и `max-h-full`
          у картинки не от чего считать — она вылезает за экран. */}
      <div
        className="flex min-h-0 flex-1 gap-2 px-3 pb-3"
        onPointerDown={(event) => event.target === event.currentTarget && close()}
      >
        {files.length > 1 && (
          <button
            type="button"
            onClick={() => step(-1)}
            aria-label={t("action.previous")}
            className={`${button} self-center`}
          >
            <Icon as={IconChevronLeft} size={20} />
          </button>
        )}

        <div
          className="flex min-h-0 min-w-0 flex-1 items-center justify-center"
          onPointerDown={(event) => event.target === event.currentTarget && close()}
        >
          <Content url={url} kind={kind ?? fileKind(url)} />
        </div>

        {files.length > 1 && (
          <button
            type="button"
            onClick={() => step(1)}
            aria-label={t("action.next")}
            className={`${button} self-center`}
          >
            <Icon as={IconChevronRight} size={20} />
          </button>
        )}
      </div>
    </Modal>
  );
}

/* Кнопки лежат на затемнении, а не на поверхности, — поэтому белые
   и без токенов темы: под ними всегда одно и то же. */
const button =
  "grid size-8 shrink-0 place-items-center rounded-md text-white/80 transition-colors hover:bg-white/15 hover:text-white";

/**
 * Сам файл. Чего браузер показать не умеет — не показываем: пустая
 * рамка с крестиком сообщает меньше, чем имя файла и кнопка «скачать».
 */
function Content({ url, kind }: { url: string; kind: FileKind }) {
  const { t } = useTranslation();

  switch (kind) {
    case "image":
      return (
        <img
          src={url}
          alt=""
          className="max-h-full max-w-full rounded-md object-contain shadow-modal"
        />
      );

    case "video":
      return <video src={url} controls autoPlay className="max-h-full max-w-full rounded-md" />;

    case "audio":
      return <audio src={url} controls autoPlay className="w-full max-w-lg" />;

    case "pdf":
      // Своей отрисовки PDF нет и не будет: у браузера она уже есть,
      // а pdf.js — это 300 КБ ради того же самого.
      return <iframe src={url} title={fileName(url)} className="size-full rounded-md bg-white" />;

    default:
      return (
        <a
          href={url}
          download
          target="_blank"
          rel="noreferrer noopener"
          className="flex flex-col items-center gap-3 rounded-lg bg-surface p-8 text-sm text-fg"
        >
          <Icon as={IconFile} size={40} className="text-fg-muted" />
          <span className="max-w-xs truncate">{fileName(url)}</span>
          <span className="text-accent-text underline">{t("cell.download")}</span>
        </a>
      );
  }
}
