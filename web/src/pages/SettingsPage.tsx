import { useAppData } from "../AppData";
import { AccountSettings } from "../components/AccountSettings";
import { Spinner } from "../ui/Spinner";

export function SettingsPage() {
  const { me, refresh, loading } = useAppData();

  return (
    <div className="py-6">
      <h1 className="mb-4 text-xl font-semibold text-ink">Settings</h1>
      {me ? <AccountSettings me={me} onChanged={refresh} /> : loading ? <Spinner label="Loading…" /> : null}
    </div>
  );
}
