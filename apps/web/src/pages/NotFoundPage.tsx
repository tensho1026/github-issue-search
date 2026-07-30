import { Compass, Home } from "lucide-react";
import { Link } from "react-router";

import { Button } from "../components/ui/button";
import { Card, CardContent } from "../components/ui/card";
import { Icon } from "../components/ui/icon";
import { appRoutes } from "../shared/config/app-config";

export function NotFoundPage() {
  return (
    <section className="mx-auto grid min-h-[68vh] w-full max-w-3xl content-center px-5 py-16 sm:px-8">
      <Card>
        <CardContent className="grid justify-items-start gap-5 p-8 sm:p-10">
          <span className="grid size-14 place-items-center rounded-2xl bg-accent-soft text-accent-soft-foreground">
            <Icon className="size-6" icon={Compass} />
          </span>
          <div>
            <p className="font-mono text-xs tracking-[0.16em] text-muted-foreground uppercase">
              404 · Route not found
            </p>
            <h1 className="mt-3 text-3xl font-semibold tracking-[-0.045em]">
              This trail ends here.
            </h1>
            <p className="mt-3 max-w-lg leading-7 text-muted-foreground">
              The page may have moved. Return home to analyze a public GitHub
              profile.
            </p>
          </div>
          <Button asChild>
            <Link to={appRoutes.home}>
              <Icon icon={Home} />
              Back to IssueScout
            </Link>
          </Button>
        </CardContent>
      </Card>
    </section>
  );
}
