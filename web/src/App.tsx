import { useEffect } from "react";
import { QueryClient, QueryClientProvider, useQuery, useQueryClient } from "@tanstack/react-query";
import { Route, Switch } from "wouter";
import { api, ApiError } from "@/lib/api";
import { ThemeProvider } from "@/hooks/use-theme";
import { ConfirmProvider } from "@/components/confirm";
import { StepUpProvider } from "@/components/guard";
import { StackRunnerProvider } from "@/components/StackRunner";
import { CommandPaletteProvider } from "@/components/CommandPalette";
import { SitesPage } from "@/pages/Sites";
import { Toaster } from "@/components/ui/toaster";
import { Layout } from "@/components/Layout";
import { Login } from "@/pages/Login";
import { OverviewPage } from "@/pages/Overview";
import { ContainersPage } from "@/pages/Containers";
import { ContainerPage } from "@/pages/Container";
import { StacksPage } from "@/pages/Stacks";
import { StackPage } from "@/pages/Stack";
import { StoragePage } from "@/pages/Storage";
import { ActivityPage } from "@/pages/Activity";
import { SettingsPage } from "@/pages/Settings";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Signed-out answers aren't worth retrying; the session check takes over.
      retry: (n, err) => !(err instanceof ApiError && err.status < 500) && n < 2,
      refetchOnWindowFocus: true,
      staleTime: 2_000,
    },
  },
});

function Gate() {
  const qc = useQueryClient();
  const session = useQuery({ queryKey: ["session"], queryFn: api.session, staleTime: 60_000 });
  useEffect(() => {
    const out = () => qc.invalidateQueries({ queryKey: ["session"] });
    addEventListener("stunsails:signed-out", out);
    return () => removeEventListener("stunsails:signed-out", out);
  }, [qc]);
  if (session.isPending) return null;
  if (!session.data?.authenticated) return <Login configured={session.data?.configured ?? true} />;
  return (
    <StepUpProvider>
      <StackRunnerProvider>
      <CommandPaletteProvider>
      <Layout>
        <Switch>
          <Route path="/" component={OverviewPage} />
          <Route path="/containers" component={ContainersPage} />
          <Route path="/containers/:id">{(p) => <ContainerPage id={p.id} />}</Route>
          <Route path="/stacks" component={StacksPage} />
          <Route path="/stacks/:project">{(p) => <StackPage project={p.project} />}</Route>
          <Route path="/sites" component={SitesPage} />
          <Route path="/storage" component={StoragePage} />
          <Route path="/activity" component={ActivityPage} />
          <Route path="/settings" component={SettingsPage} />
          <Route>
            <p className="text-sm text-muted-foreground">There's no page here.</p>
          </Route>
        </Switch>
      </Layout>
      </CommandPaletteProvider>
      </StackRunnerProvider>
    </StepUpProvider>
  );
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <ConfirmProvider>
          <Gate />
          <Toaster />
        </ConfirmProvider>
      </ThemeProvider>
    </QueryClientProvider>
  );
}
