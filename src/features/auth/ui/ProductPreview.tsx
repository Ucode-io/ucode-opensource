import { Chip } from "@/shared/ui/chip";

/**
 * Макет продукта на панели входа. Не скриншот и не картинка — та же
 * вёрстка на тех же токенах, что и настоящая таблица. Значит, он не
 * устареет молча: поменяются токены — поменяется и превью.
 *
 * Данные английские и выдуманные. Внутри aria-hidden панели, поэтому
 * скринридер их не читает.
 */

const ROWS = [
  { name: "Brandon Clark", company: "airbnb.com", status: "In progress", color: "blue", value: "24,000" },
  { name: "Mia Rodriguez", company: "figma.com", status: "Done", color: "green", value: "8,300" },
  { name: "Ryan Mitchell", company: "dropbox.com", status: "On review", color: "yellow", value: "12,500" },
  { name: "Sarah Reynolds", company: "notion.so", status: "Not started", color: "gray", value: "4,200" },
  { name: "David Larson", company: "stripe.com", status: "In progress", color: "blue", value: "31,000" },
  { name: "Emma Thompson", company: "slack.com", status: "Done", color: "green", value: "16,750" },
] as const;

export function ProductPreview() {
  return (
    <div className="w-[min(560px,100%)] overflow-hidden rounded-xl border border-border bg-surface shadow-modal">
      <header className="flex items-center justify-between border-b border-border px-3 py-2.5">
        <div className="flex items-center gap-2">
          <TableIcon />
          <span className="text-sm font-medium text-fg">Customers</span>
          <span className="text-xs text-fg-subtle">· 128</span>
        </div>
        <div className="flex items-center gap-1.5">
          <span className="rounded-sm bg-surface-active px-1.5 py-0.5 text-2xs text-fg-muted">
            All customers
          </span>
          <span className="rounded-sm bg-accent-solid px-2 py-0.5 text-2xs font-medium text-accent-fg">
            New
          </span>
        </div>
      </header>

      <table className="w-full border-collapse text-left">
        <thead>
          <tr className="text-xs text-fg-muted">
            <th className="h-8 px-3 font-normal">Name</th>
            <th className="h-8 px-3 font-normal">Company</th>
            <th className="h-8 px-3 font-normal">Status</th>
            <th className="h-8 px-3 text-right font-normal">Deal size</th>
          </tr>
        </thead>
        <tbody>
          {ROWS.map((row) => (
            <tr key={row.name} className="border-t border-border">
              <td className="h-9 px-3">
                <span className="flex items-center gap-2">
                  <span className="grid size-5 place-items-center rounded-full bg-accent-subtle text-2xs font-semibold text-accent-text">
                    {row.name[0]}
                  </span>
                  <span className="text-sm text-fg">{row.name}</span>
                </span>
              </td>
              <td className="h-9 px-3">
                <span className="rounded-sm bg-surface-active px-1.5 py-0.5 text-xs text-fg-muted">
                  {row.company}
                </span>
              </td>
              <td className="h-9 px-3">
                <Chip color={row.color}>{row.status}</Chip>
              </td>
              <td className="h-9 px-3 text-right text-sm tabular-nums text-fg">${row.value}</td>
            </tr>
          ))}
        </tbody>
      </table>

      <footer className="border-t border-border px-3 py-2 text-xs text-fg-subtle">
        6 of 128 records
      </footer>
    </div>
  );
}

function TableIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" className="text-fg-muted">
      <rect x="2" y="2.5" width="12" height="11" rx="1.5" />
      <path d="M2 6h12M6 6v7.5" />
    </svg>
  );
}
