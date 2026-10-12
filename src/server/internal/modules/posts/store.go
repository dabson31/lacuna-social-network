package posts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var errNotVisible = errors.New("post not found")

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// visible is the one visibility rule for the whole feature, expressed as sql so the
// database does the filtering, doing it in go would break pagination because we would
// ask for twenty rows and hand back only the few that survived the filter, the viewer
// id is passed three times, once per branch, all the ? in order are the same viewer

// visibility rule for the entire thing, and the filtering is done through the database (sql), as doing
// it in go is not viable because it would break pagination (slows down computation and drains memory, bloaaaat)
// because of the filter removing most of what we'll fetch anyway, the viewer id has to be passed three times, once
// every branch, all the ? = the same viewer
//
// ! NAMED parameters, not positional, and that is load bearing rather than tidy. this predicate
// ! is pasted into four different statements, each of which puts its OWN arguments around it in
// ! a different order — canSee even passes the post id BEFORE the viewer. every branch needs the
// ! same viewer, so with ? the caller has to count the branches and repeat the id exactly that
// ! many times, in exactly the right slots. adding the follow check below took the count from
// ! three to four, which would have silently shifted the cursor, the limit and the author id by
// ! one in three of those four call sites. a named parameter is bound by name, so a new branch
// ! costs nothing and cannot reach the arguments around it
// ? sqlite refuses to mix the two styles in one statement, which is why the callers' own
// ? arguments are named too
const visible = `(
	p.author_id = :viewer
	OR p.privacy = 'public'
	OR (p.privacy = 'almost_private' AND EXISTS (
		SELECT 1 FROM follows f
		WHERE f.follower_id = :viewer AND f.followee_id = p.author_id AND f.status = 'accepted'))
	OR (p.privacy = 'private'
	    AND EXISTS (
		SELECT 1 FROM post_viewers pv
		WHERE pv.post_id = p.id AND pv.user_id = :viewer)
	    --! being on the list is no longer enough on its own. the author picks from their
	    --! followers, and the pick is a NARROWING of that audience rather than a grant that
	    --! outlives it, so the follow has to still be there when the post is read
	    --! without this the three levels are not a ladder: an ex follower is refused an
	    --! almost_private post and handed a private one, which makes the strictest setting the
	    --! leakiest. the row in post_viewers was written once, at publish, and nothing has ever
	    --! gone back to it — unfollow does not know the table exists
	    --? read time and not unfollow time, deliberately. deleting the row would quietly edit
	    --? the author's chosen list on their behalf, and it would make a reconciliation
	    --? permanent: follow again and the post is theirs to read again, which is what somebody
	    --? who re-follows expects
	    AND EXISTS (
		SELECT 1 FROM follows f
		WHERE f.follower_id = :viewer AND f.followee_id = p.author_id AND f.status = 'accepted'))
)`

// the column list and the author join are shared by every read so the scanner matches
// this creates a single reusable pipeline for posts, we always receieve the columns with the
// exact same order listed below, selectPost is the blueprint/package, and the scanPost is basically
// the unpacker that receives the data in the order it wants exactly and maps it accordingly
const selectPost = ` 
	SELECT p.id, p.author_id, p.content, p.image_path, p.privacy, p.created_at,
	       u.first_name || ' ' || u.last_name AS author_name, u.avatar_path,
	       (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id) AS comment_count,
	       --! counted the same way the private branch of the visibility rule grants access, so the
	       --! number and the rule cannot disagree. the card renders it as "selected · N" and
	       --! viewers button says "visible to N selected accounts" — that second one is a claim
	       --! about who can READ the post, and a bare count of post_viewers stopped being an
	       --! answer to it the moment a lapsed follow started revoking access
	       --? it is the author's own reach readout, so it follows reach: pick three, one walks
	       --? away, it reads 2, and it reads 3 again if they come back
	       (SELECT COUNT(*) FROM post_viewers pv2
	         WHERE pv2.post_id = p.id
	           AND EXISTS (SELECT 1 FROM follows f2
	                        WHERE f2.follower_id = pv2.user_id
	                          AND f2.followee_id = p.author_id
	                          AND f2.status = 'accepted')) AS viewer_count
	FROM posts p
	JOIN users u ON u.id = p.author_id`

func scanPost(rows *sql.Rows) (Post, error) {
	var p Post
	err := rows.Scan(&p.ID, &p.AuthorID, &p.Content, &p.ImagePath, &p.Privacy, &p.CreatedAt,
		&p.AuthorName, &p.AuthorAvatar, &p.CommentCount, &p.ViewerCount)
	return p, err
}

// creates the post
func (s *Store) create(ctx context.Context, authorID int64, content string, imagePath *string, privacy string, viewerIDs []int64) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	// rollback is a no op after a successful commit, so this is always safe to defer
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO posts (author_id, content, image_path, privacy) VALUES (?, ?, ?, ?)`,
		authorID, content, imagePath, privacy,
	)
	if err != nil {
		return 0, fmt.Errorf("insert post: %w", err)
	}
	postID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if privacy == PrivacyPrivate {
		for _, uid := range viewerIDs {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO post_viewers (post_id, user_id) VALUES (?, ?)`, postID, uid); err != nil {
				return 0, fmt.Errorf("insert viewer: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return postID, nil
}

// this returns which of the given ids are accepeted followers of current user (author), and
// then the service compares the len against what was sent to figure out any id that is not
// a real follower so priv posts cant be shared with strangers
func (s *Store) acceptedAmong(ctx context.Context, authorID int64, ids []int64) (map[int64]bool, error) {
	if len(ids) == 0 {
		return map[int64]bool{}, nil
	}
	// build one placeholder per id for the IN list
	marks := strings.Repeat("?,", len(ids))
	marks = marks[:len(marks)-1]
	args := make([]any, 0, len(ids)+1)
	args = append(args, authorID)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT follower_id FROM follows
		 WHERE followee_id = ? AND status = 'accepted' AND follower_id IN (`+marks+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("accepted among: %w", err)
	}
	defer rows.Close()

	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// this makes sure the newest posts are visible first and shows the feed obv
func (s *Store) feed(ctx context.Context, viewerID, before int64, limit int) ([]Post, error) {
	return s.query(ctx,
		selectPost+` WHERE `+visible+` AND (:before = 0 OR p.id < :before) ORDER BY p.id DESC LIMIT :limit`,
		sql.Named("viewer", viewerID), sql.Named("before", before), sql.Named("limit", limit))
}

// this is the exact same thing as feeds, but filtered to one author while following visibility rules
func (s *Store) authorPosts(ctx context.Context, viewerID, authorID, before int64, limit int) ([]Post, error) {
	return s.query(ctx,
		selectPost+` WHERE `+visible+` AND p.author_id = :author AND (:before = 0 OR p.id < :before)
		 ORDER BY p.id DESC LIMIT :limit`,
		sql.Named("viewer", viewerID), sql.Named("author", authorID),
		sql.Named("before", before), sql.Named("limit", limit))
}

func (s *Store) query(ctx context.Context, q string, args ...any) ([]Post, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query posts: %w", err)
	}
	defer rows.Close()

	var out []Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// this returns a single post only if the viewer may see it, a post they cannot see is
// reported as not found rather than forbidden, a 403 would confirm the post exists
// so its like a story in private instagram profile
func (s *Store) byID(ctx context.Context, viewerID, postID int64) (Post, error) {
	rows, err := s.db.QueryContext(ctx,
		selectPost+` WHERE `+visible+` AND p.id = :post`,
		sql.Named("viewer", viewerID), sql.Named("post", postID))
	if err != nil {
		return Post{}, fmt.Errorf("post by id: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return Post{}, errNotVisible
	}
	p, err := scanPost(rows)
	if err != nil {
		return Post{}, fmt.Errorf("scan post: %w", err)
	}
	return p, nil
}

// canSee is a cheap existence check used before adding a comment, it reuses the same
// rule without pulling the whole row back
func (s *Store) canSee(ctx context.Context, viewerID, postID int64) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM posts p WHERE p.id = :post AND `+visible,
		sql.Named("post", postID), sql.Named("viewer", viewerID),
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("can see: %w", err)
	}
	return true, nil
}

func (s *Store) addComment(ctx context.Context, postID, authorID int64, content string, imagePath *string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO comments (post_id, author_id, content, image_path) VALUES (?, ?, ?, ?)`,
		postID, authorID, content, imagePath,
	)
	if err != nil {
		return 0, fmt.Errorf("insert comment: %w", err)
	}
	return res.LastInsertId()
}

// this one is oldest to newest top to bottom following standard practice
func (s *Store) comments(ctx context.Context, postID int64) ([]Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.post_id, c.author_id, c.content, c.image_path, c.created_at,
		        u.first_name || ' ' || u.last_name, u.avatar_path
		 FROM comments c JOIN users u ON u.id = c.author_id
		 WHERE c.post_id = ? ORDER BY c.id ASC`, postID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	var out []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.PostID, &c.AuthorID, &c.Content, &c.ImagePath, &c.CreatedAt, &c.AuthorName, &c.AuthorAvatar); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// imagePaths collects every file this post owns, its own image plus one per comment, so the
// service can unlink them after the row is gone
// ! the rows have to be read BEFORE the delete, because the cascade takes the comments with
// ! the post and then there is nothing left to ask. collecting afterwards leaves every comment
// ! image orphaned on disk forever, which is the kind of leak nobody notices until the volume
// ! fills up
func (s *Store) imagePaths(ctx context.Context, postID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT image_path FROM posts WHERE id = ? AND image_path IS NOT NULL
		 UNION ALL
		 SELECT image_path FROM comments WHERE post_id = ? AND image_path IS NOT NULL`,
		postID, postID)
	if err != nil {
		return nil, fmt.Errorf("post image paths: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan image path: %w", err)
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// remove deletes a post, but only if the caller wrote it.
// ! the author check is in the WHERE and not in a separate SELECT, so there is no window
// ! between "is it yours" and "delete it". comments and post_viewers go with it through
// ! ON DELETE CASCADE, which only works because Open() sets foreign_keys = ON
func (s *Store) remove(ctx context.Context, authorID, postID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM posts WHERE id = ? AND author_id = ?`, postID, authorID)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	// nothing matched, so either the post is gone or it belongs to somebody else. both read as
	// not found, we never confirm that someone else's post exists to a caller who cannot touch it
	if n == 0 {
		return errNotVisible
	}
	return nil
}
