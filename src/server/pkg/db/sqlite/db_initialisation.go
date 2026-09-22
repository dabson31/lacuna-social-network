package sqlite

var DB *sql.DB

func initDB() {
	var err error 

	DB, err = sql.Open("sqlite3", "./modules/users/social_network.db")
	if err != nil {
		log.Fatal(fmt.Errorf("Error: failed to open database: %v", err))
	}




	if err := ensureUsersTable(); err != nil {
		log.Fatal(fmt.Errorf("Error: could not initialise users table: %v", err))
	}


		createUsersDB := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		password text NOT NULL,
		first_name text NOT NULL,
		last_name text NOT NULL,
		date_of_birth TEXT NOT NULL, -- Format: 'DD-MM-YYYY'
		avatar_url TEXT,
		nickname TEXT,
		about_me TEXT
	);
	`

	_, err := DB.Exec(createUsersDB)
	if err != nil {
		log.Fatal(fmt.Errorf("could not create user table: %v", err))
	}	

	createPostsDB := `
	CREATE TABLE IF NOT EXISTS posts (
		post_id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER, 
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		image VARCHAR(255), 
		FOREIGN KEY (user_id) REFERENCES users (id)
		);
		`



	_, err = DB.Exec(createPostsDB)
	if err != nil {
		log.Fatal(fmt.Errorf("Error: could not create post's table."))
	}





		createCategoryDB := `
	CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		name TEXT NOT NULL UNIQUE
	);  
	`



	_, err = DB.Exec(createCategoryDB)
	if err != nil {
		log.Fatal(fmt.Errorf("Error: could not create category's table."))
	}



	createPostCategoriesDB := `
	CREATE TABLE IF NOT EXISTS post_categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id INTEGER NOT NULL,
		category_id INTEGER NOT NULL,
		UNIQUE(post_id, category_id),
		FOREIGN KEY (post_id) REFERENCES posts (post_id),
		FOREIGN KEY (category_id) REFERENCES categories (id)
	);
	`


	_, err = DB.Exec(createPostCategoriesDB)
	if err != nil {
		log.Fatal(fmt.Errorf("Error: could not create post_categories table"))
	}



		createCommentsDB := `
	CREATE TABLE IF NOT EXISTS comments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		post_id INTEGER NOT NULL,
		content TEXT NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users (id),
		FOREIGN KEY (post_id) REFERENCES posts (post_id)
	);
	`
	_, err = DB.Exec(createCommentsDB)
	if err != nil {
		log.Fatal(fmt.Errorf("Error: could not create comments's table."))
	}

	createLikeDB := `
	CREATE TABLE IF NOT EXISTS likes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		post_id INTEGER NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users (id),
		FOREIGN KEY (post_id) REFERENCES posts (post_id)
		);
		`



	_, err = DB.Exec(createLikeDB)
	if err != nil {
		log.Fatal(fmt.Errorf("Error: could not create like's table."))
	}


	createDislikeDB := `
	CREATE TABLE IF NOT EXISTS dislike (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		post_id INTEGER,
		FOREIGN KEY (user_id) REFERENCES users (id),
		FOREIGN KEY (post_id) REFERENCES posts (post_id)
		);
		`



	_, err = DB.Exec(createDislikeDB)
	if err != nil {
		log.Fatal(fmt.Errorf("Error: could not create dislike's table."))
	}


	// comments_like table holds likes on comments
	createCommentLikeDB := `
	CREATE TABLE IF NOT EXISTS comments_like (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		comment_id INTEGER,
		FOREIGN KEY (user_id) REFERENCES users (id),
		FOREIGN KEY (comment_id) REFERENCES comments (id)
	);
	`



	_, err = DB.Exec(createCommentLikeDB)
	if err != nil {
		log.Fatal(fmt.Errorf("Error: could not create comments_like's table."))
	}



	createCommentDislikeDB := `
	CREATE TABLE IF NOT EXISTS comments_dislike (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		comment_id INTEGER,
		FOREIGN KEY (user_id) REFERENCES users (id),
		FOREIGN KEY (comment_id) REFERENCES comments (id)
	);
	`



	_, err = DB.Exec(createCommentDislikeDB)
	if err != nil {
		log.Fatal(fmt.Errorf("Error: could not create comment_dislike's table."))
	}


	createSessionsDB := `
	CREATE TABLE IF NOT EXISTS sessions (
	    id TEXT PRIMARY KEY,
	    user_id INTEGER NOT NULL,
	    token TEXT NOT NULL,
	    expires_at DATETIME NOT NULL,
	    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	    FOREIGN KEY (user_id) REFERENCES users (id)
	);`
	



	columns, err := tableColumns("users")
	if err != nil {
		log.Fatal(err)
	}

		if !hasColumn(columns, "email") {
		if _, err := DB.Exec(`ALTER TABLE users ADD COLUMN email TEXT NOT NULL UNIQUE DEFAULT ''`); err != nil {
			log.Fatal(err)
		}
	}

	if !hasColumn(columns, "password") {
		if _, err := DB.Exec(`ALTER TABLE users ADD COLUMN password TEXT NOT NULL DEFAULT ''`); err != nil {
			log.Fatal(err)
		}
	}

	if !hasColumn(columns, "first_name") {
		if _, err := DB.Exec(`ALTER TABLE users ADD COLUMN first_name TEXT NOT NULL DEFAULT ''`); err != nil {
			log.Fatal(err)
		}
	}

	if !hasColumn(columns, "last_name") {
		if _, err := DB.Exec(`ALTER TABLE users ADD COLUMN last_name TEXT NOT NULL DEFAULT ''`); err != nil {
			log.Fatal(err)
		}
	}

	if !hasColumn(columns, "date_of_birth") {
		if _, err := DB.Exec(`ALTER TABLE users ADD COLUMN date_of_birth TEXT NOT NULL DEFAULT ''`); err != nil {
			log.Fatal(err)
		}
	}

	if !hasColumn(columns, "avatar_url") {
		if _, err := DB.Exec(`ALTER TABLE users ADD COLUMN avatar_url TEXT`); err != nil {
			log.Fatal(err)
		}
	}

	if !hasColumn(columns, "nickname") {
		if _, err := DB.Exec(`ALTER TABLE users ADD COLUMN nickname TEXT`); err != nil {
			log.Fatal(err)
		}
	}

	if !hasColumn(columns, "about_me") {
		if _, err := DB.Exec(`ALTER TABLE users ADD COLUMN about_me TEXT`); err != nil {
			log.Fatal(err)
		}
	}
}

func tableColumns(tableName string) ([]string, error) {
	rows, err := DB.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make([]string, 0)
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, nil
}

func hasColumn(columns []string, target string) bool {
	for _, column := range columns {
		if column == target {
			return true
		}
	}
	return false
}