import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

type AnalysisPlaceholderTabProps = {
  title: string;
  description: string;
};

export function AnalysisPlaceholderTab({ title, description }: AnalysisPlaceholderTabProps) {
  return (
    <Card className="flex min-h-[200px] flex-col items-center justify-center p-6 text-center">
      <Badge variant="info">준비 중</Badge>
      <h3 className="mt-3 text-sm font-semibold text-[var(--text-primary)]">{title}</h3>
      <p className="mt-2 max-w-md text-cell leading-relaxed text-[var(--text-muted)]">{description}</p>
    </Card>
  );
}
