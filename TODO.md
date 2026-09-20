# TODO

## Authentication

- [ ] Add watched_movie_id in session
- [ ] Add a proper refresh flow (or explicitly keep sliding sessions and document that choice)
- [ ] Add session revocation and session management (current session, specific session, all sessions)
- [ ] Add password lifecycle flows (change password, reset password, invalidate sessions after reset)
- [ ] Add absolute session lifetime and secret rotation support
- [ ] Add a real authorization model where needed (roles/scopes/permissions)
- [ ] There is no way to get your userID. Add something like auth/me
- [ ] I think we miss a movie + user_movie_list tables. You can say by watching the session table. I had to remove watched movie ID foreign key as there might be movie_id multiple time in movie table as the same movie can be added by different users
