import { normalizeIssueBody } from "../model/safe-issue-body";

export type SafeIssueBodyProps = {
  body: string;
};

export function SafeIssueBody({ body }: SafeIssueBodyProps) {
  const content = normalizeIssueBody(body);
  return (
    <div>
      <p className="mb-3 text-xs leading-5 text-muted-foreground">
        GitHub Markdown is shown as safe plain text. Links and embedded content
        are not activated.
      </p>
      <pre
        className="max-h-[38rem] overflow-auto rounded-xl border border-border bg-muted/35 p-4 font-sans text-sm leading-6 whitespace-pre-wrap wrap-break-word"
        data-truncated={content.truncated}
      >
        {content.text}
      </pre>
    </div>
  );
}
