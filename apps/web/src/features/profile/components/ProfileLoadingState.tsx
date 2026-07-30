import { Card, CardContent, CardHeader } from "../../../components/ui/card";
import { Skeleton } from "../../../components/ui/skeleton";

export function ProfileLoadingState() {
  return (
    <div
      aria-label="Analyzing public GitHub profile"
      className="mx-auto grid min-h-[70vh] w-full max-w-7xl content-center gap-6 px-5 py-12 sm:px-8 lg:px-10"
      role="status"
    >
      <div className="grid gap-3">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-12 w-full max-w-xl" />
        <Skeleton className="h-6 w-full max-w-2xl" />
      </div>
      <div className="grid gap-5 lg:grid-cols-[0.8fr_1.2fr]">
        <Card>
          <CardHeader className="flex-row items-center gap-4">
            <Skeleton className="size-20 rounded-2xl" />
            <div className="grid flex-1 gap-3">
              <Skeleton className="h-6 w-1/2" />
              <Skeleton className="h-4 w-3/4" />
            </div>
          </CardHeader>
          <CardContent className="grid grid-cols-3 gap-3">
            {Array.from({ length: 3 }, (_, index) => (
              <Skeleton className="h-18" key={index} />
            ))}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <Skeleton className="h-7 w-52" />
          </CardHeader>
          <CardContent className="grid gap-5">
            {Array.from({ length: 4 }, (_, index) => (
              <div className="grid gap-2" key={index}>
                <Skeleton className="h-4 w-28" />
                <Skeleton className="h-3 w-full" />
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
      <span className="sr-only">
        Reading profile and bounded repository evidence…
      </span>
    </div>
  );
}
