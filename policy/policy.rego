package supabase.kimi

# Allow insert only if user_id == auth.uid()
allow_insert {
	input.table == "users_pages"
	input.data.user_id == input.auth.uid
}
