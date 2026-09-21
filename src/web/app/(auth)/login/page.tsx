import { redirect } from "next/navigation";
import { getSession, login, logout } from "../../../lib/session/session";

export default async function Page() {
    const session = await getSession();
    return (
        <section>
            <h1> TESTING ALEP </h1>
            <form action ={async (formData) => {
                'use server';
                await login(formData);
                redirect('/login');
            }}
            >
                <input type="email" name="email" placeholder="Email" />
                <br />
                <button type="submit">Login</button>
            </form>
            <form
            action ={async () => {
                'use server';
                await logout();
                redirect('/login');
            }}
            >
                <button type="submit">Logout</button>
            </form>
            <h2> This should be the session: </h2>
            <pre>{JSON.stringify(session, null, 2)}</pre>
        </section>
    );
}