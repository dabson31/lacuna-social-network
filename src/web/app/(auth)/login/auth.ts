import NextAuth from "next-auth";
import GitHub from "next-auth/providers/github";


const GITHUB_CLIENT_ID = process.env.GITHUB_CLIENT_ID;
const GITHUB_CLIENT_SECRET = process.env.GITHUB_CLIENT_SECRET;

if(!GITHUB_CLIENT_ID || !GITHUB_CLIENT_SECRET) {
    throw new Error("github variables missing.")
}

export const {handlers:{GET, POST}, auth, sign, signIn}= NextAuth({
    // use some sort of adapter, instead of prisma use our sqlite
    // something like "DrizzleAdapter(db)"
    // for reference, an adapter is the layer that tells this auth.ts how to persist/store users,acc,sessions,tokens etc 
    // into our database
    providers: [
        GitHub({
            clientId: GITHUB_CLIENT_ID,
            clientSecret: GITHUB_CLIENT_SECRET,
        }),
    ],
    callbacks:{
        async session ({session, user}: any) {
            if(session && user) {
                session.user.id = user.id;
            }
            return session;
        },
    },

    
});