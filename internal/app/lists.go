package app

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/rom/samling/internal/crypto"
	"github.com/rom/samling/internal/db"
)

func itoa(n int) string { return strconv.Itoa(n) }

// deleteRes removes a resource row together with its shares.
func (a *App) deleteRes(ctx context.Context, resType, id string) error {
	return a.DB.InTx(ctx, func(tx *db.Tx) error {
		if err := tx.Exec(ctx, `DELETE FROM acl WHERE res_type=? AND res_id=?`, resType, id); err != nil {
			return err
		}
		return tx.Exec(ctx, `DELETE FROM `+resTables[resType]+` WHERE id=?`, id)
	})
}

// ---------- lists ----------

type ListItem struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Done      bool   `json:"done"`
	Priority  int    `json:"priority"` // 0 none, 1 low, 2 medium, 3 high
	Position  int    `json:"position"`
	DueAt     int64  `json:"due_at"`
	CreatedBy string `json:"created_by"`
	CreatedAt int64  `json:"created_at"`
}

type List struct {
	ID        string     `json:"id"`
	OwnerID   string     `json:"owner_id"`
	Title     string     `json:"title"`
	Kind      string     `json:"kind"` // todo | priority | plain
	CreatedAt int64      `json:"created_at"`
	UpdatedAt int64      `json:"updated_at"`
	Shared    bool       `json:"shared"`
	Total     int        `json:"total"`
	Open      int        `json:"open"`
	Level     int        `json:"level,omitempty"`
	Items     []ListItem `json:"items"`
}

var listKinds = map[string]bool{"todo": true, "priority": true, "plain": true}

func (a *App) hLists(c *Ctx) error {
	vis, args := visible(c.U, "list", "l")
	out := []List{}
	err := a.DB.Each(c.ctx(), `SELECT l.id, l.owner_id, l.title, l.kind, l.created_at, l.updated_at,
		(SELECT COUNT(*) FROM acl a WHERE a.res_type='list' AND a.res_id=l.id),
		(SELECT COUNT(*) FROM list_items i WHERE i.list_id=l.id),
		(SELECT COUNT(*) FROM list_items i WHERE i.list_id=l.id AND i.done=0)
		FROM lists l WHERE `+vis+` ORDER BY l.updated_at DESC`, args, func(r *sql.Rows) error {
		var l List
		var shares int
		if err := r.Scan(&l.ID, &l.OwnerID, &l.Title, &l.Kind, &l.CreatedAt, &l.UpdatedAt, &shares, &l.Total, &l.Open); err != nil {
			return err
		}
		l.Shared = shares > 0
		out = append(out, l)
		return nil
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

func (a *App) hCreateList(c *Ctx) error {
	var in struct {
		Title  string  `json:"title"`
		Kind   string  `json:"kind"`
		Shares []Share `json:"shares"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if in.Title = clean(in.Title, 120); in.Title == "" {
		return errBad("title_required")
	}
	if !listKinds[in.Kind] {
		return errBad("invalid_kind")
	}
	shares, err := a.createShares(c, in.Shares)
	if err != nil {
		return err
	}
	if err := limitCount(c.ctx(), a.DB, maxPerOwner, `SELECT COUNT(*) FROM lists WHERE owner_id=?`, c.U.ID); err != nil {
		return err
	}
	id, now := crypto.NewID(), a.unix()
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `INSERT INTO lists (id,owner_id,title,kind,created_at,updated_at) VALUES (?,?,?,?,?,?)`, id, c.U.ID, in.Title, in.Kind, now, now); err != nil {
			return err
		}
		return setShares(c.ctx(), tx, "list", id, shares)
	})
	if err != nil {
		return err
	}
	a.logAct(c, false, "list.created", "list", id, in.Title)
	return c.JSON(map[string]string{"id": id})
}

func (a *App) hList(c *Ctx) error {
	id := c.Param("id")
	lvl, err := a.level(c.ctx(), c.U, "list", id)
	if err != nil || lvl < LevelRead {
		return errNotFound
	}
	l := List{Level: lvl, Items: []ListItem{}}
	if err := a.DB.Row(c.ctx(), `SELECT id, owner_id, title, kind, created_at, updated_at FROM lists WHERE id=?`, id).
		Scan(&l.ID, &l.OwnerID, &l.Title, &l.Kind, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return err
	}
	err = a.DB.Each(c.ctx(), `SELECT id, text, done, priority, position, due_at, created_by, created_at FROM list_items WHERE list_id=? ORDER BY position, created_at`,
		[]any{id}, func(r *sql.Rows) error {
			var i ListItem
			if err := r.Scan(&i.ID, &i.Text, &i.Done, &i.Priority, &i.Position, &i.DueAt, &i.CreatedBy, &i.CreatedAt); err != nil {
				return err
			}
			l.Items = append(l.Items, i)
			return nil
		})
	if err != nil {
		return err
	}
	return c.JSON(l)
}

func (a *App) hUpdateList(c *Ctx) error {
	id := c.Param("id")
	var in struct {
		Title string `json:"title"`
		Kind  string `json:"kind"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.need(c, "list", id, LevelManage); err != nil {
		return err
	}
	if in.Title = clean(in.Title, 120); in.Title == "" {
		return errBad("title_required")
	}
	if !listKinds[in.Kind] {
		return errBad("invalid_kind")
	}
	if err := a.DB.Exec(c.ctx(), `UPDATE lists SET title=?, kind=?, updated_at=? WHERE id=?`, in.Title, in.Kind, a.unix(), id); err != nil {
		return err
	}
	a.hub.publish(Event{"list", id})
	return c.OK()
}

func (a *App) hDeleteList(c *Ctx) error {
	id := c.Param("id")
	if err := a.need(c, "list", id, LevelManage); err != nil {
		return err
	}
	if err := a.deleteRes(c.ctx(), "list", id); err != nil {
		return err
	}
	a.logAct(c, false, "list.deleted", "list", id, "")
	return c.OK()
}

type itemIn struct {
	Text     *string `json:"text"`
	Done     *bool   `json:"done"`
	Priority *int    `json:"priority"`
	Position *int    `json:"position"`
	DueAt    *int64  `json:"due_at"`
}

func (a *App) hAddItem(c *Ctx) error {
	id := c.Param("id")
	var in itemIn
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.need(c, "list", id, LevelWrite); err != nil {
		return err
	}
	if in.Text == nil || clean(*in.Text, 500) == "" {
		return errBad("text_required")
	}
	prio, due := 0, int64(0)
	if in.Priority != nil {
		prio = min(max(*in.Priority, 0), 3)
	}
	if in.DueAt != nil {
		due = *in.DueAt
	}
	if !validTime(due) {
		return errBad("invalid_time")
	}
	if err := limitCount(c.ctx(), a.DB, maxItemsPerList, `SELECT COUNT(*) FROM list_items WHERE list_id=?`, id); err != nil {
		return err
	}
	iid, now := crypto.NewID(), a.unix()
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		var pos int
		if err := tx.Row(c.ctx(), `SELECT COALESCE(MAX(position),0)+1 FROM list_items WHERE list_id=?`, id).Scan(&pos); err != nil {
			return err
		}
		if err := tx.Exec(c.ctx(), `INSERT INTO list_items (id,list_id,text,priority,position,due_at,created_by,created_at) VALUES (?,?,?,?,?,?,?,?)`,
			iid, id, clean(*in.Text, 500), prio, pos, due, c.U.ID, now); err != nil {
			return err
		}
		return tx.Exec(c.ctx(), `UPDATE lists SET updated_at=? WHERE id=?`, now, id)
	})
	if err != nil {
		return err
	}
	a.hub.publish(Event{"list", id})
	return c.JSON(map[string]string{"id": iid})
}

func (a *App) hUpdateItem(c *Ctx) error {
	id, iid := c.Param("id"), c.Param("item")
	var in itemIn
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.need(c, "list", id, LevelWrite); err != nil {
		return err
	}
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		ctx := c.ctx()
		var n int
		if err := tx.Row(ctx, `SELECT COUNT(*) FROM list_items WHERE id=? AND list_id=?`, iid, id).Scan(&n); err != nil || n == 0 {
			return errNotFound
		}
		if in.Text != nil && clean(*in.Text, 500) != "" {
			if err := tx.Exec(ctx, `UPDATE list_items SET text=? WHERE id=?`, clean(*in.Text, 500), iid); err != nil {
				return err
			}
		}
		if in.Done != nil {
			if err := tx.Exec(ctx, `UPDATE list_items SET done=? WHERE id=?`, b2i(*in.Done), iid); err != nil {
				return err
			}
		}
		if in.Priority != nil {
			if err := tx.Exec(ctx, `UPDATE list_items SET priority=? WHERE id=?`, min(max(*in.Priority, 0), 3), iid); err != nil {
				return err
			}
		}
		if in.DueAt != nil {
			if !validTime(*in.DueAt) {
				return errBad("invalid_time")
			}
			if err := tx.Exec(ctx, `UPDATE list_items SET due_at=? WHERE id=?`, *in.DueAt, iid); err != nil {
				return err
			}
		}
		if in.Position != nil {
			if err := reorder(ctx, tx, "list_items", "list_id", id, iid, *in.Position); err != nil {
				return err
			}
		}
		return tx.Exec(ctx, `UPDATE lists SET updated_at=? WHERE id=?`, a.unix(), id)
	})
	if err != nil {
		return err
	}
	a.hub.publish(Event{"list", id})
	return c.OK()
}

func (a *App) hDeleteItem(c *Ctx) error {
	id, iid := c.Param("id"), c.Param("item")
	if err := a.need(c, "list", id, LevelWrite); err != nil {
		return err
	}
	if err := a.DB.Exec(c.ctx(), `DELETE FROM list_items WHERE id=? AND list_id=?`, iid, id); err != nil {
		return err
	}
	a.hub.publish(Event{"list", id})
	return c.OK()
}

// reorder moves row id to the zero based index pos among the rows of the
// same parent and renumbers all positions.
func reorder(ctx context.Context, tx *db.Tx, table, parentCol, parent, id string, pos int) error {
	var ids []string
	err := tx.Each(ctx, `SELECT id FROM `+table+` WHERE `+parentCol+`=? ORDER BY position, id`, []any{parent}, func(r *sql.Rows) error {
		var x string
		if err := r.Scan(&x); err != nil {
			return err
		}
		if x != id {
			ids = append(ids, x)
		}
		return nil
	})
	if err != nil {
		return err
	}
	pos = min(max(pos, 0), len(ids))
	ids = append(ids[:pos], append([]string{id}, ids[pos:]...)...)
	for i, x := range ids {
		if err := tx.Exec(ctx, `UPDATE `+table+` SET position=? WHERE id=?`, i+1, x); err != nil {
			return err
		}
	}
	return nil
}

// ---------- kanban boards ----------

type Card struct {
	ID          string `json:"id"`
	ColumnID    string `json:"column_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	AssigneeID  string `json:"assignee_id"`
	DueAt       int64  `json:"due_at"`
	Position    int    `json:"position"`
	CreatedBy   string `json:"created_by"`
	UpdatedAt   int64  `json:"updated_at"`
}

type Column struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Position int    `json:"position"`
}

type Board struct {
	ID          string   `json:"id"`
	OwnerID     string   `json:"owner_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	UpdatedAt   int64    `json:"updated_at"`
	Shared      bool     `json:"shared"`
	Cards       int      `json:"card_count"`
	Level       int      `json:"level,omitempty"`
	Columns     []Column `json:"columns"`
	CardList    []Card   `json:"cards"`
}

func (a *App) hBoards(c *Ctx) error {
	vis, args := visible(c.U, "board", "b")
	out := []Board{}
	err := a.DB.Each(c.ctx(), `SELECT b.id, b.owner_id, b.title, b.description, b.updated_at,
		(SELECT COUNT(*) FROM acl a WHERE a.res_type='board' AND a.res_id=b.id),
		(SELECT COUNT(*) FROM cards k WHERE k.board_id=b.id)
		FROM boards b WHERE `+vis+` ORDER BY b.updated_at DESC`, args, func(r *sql.Rows) error {
		var b Board
		var shares int
		if err := r.Scan(&b.ID, &b.OwnerID, &b.Title, &b.Description, &b.UpdatedAt, &shares, &b.Cards); err != nil {
			return err
		}
		b.Shared = shares > 0
		out = append(out, b)
		return nil
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

func (a *App) hCreateBoard(c *Ctx) error {
	var in struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Columns     []string `json:"columns"`
		Shares      []Share  `json:"shares"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if in.Title = clean(in.Title, 120); in.Title == "" {
		return errBad("title_required")
	}
	if len(in.Columns) == 0 || len(in.Columns) > 12 {
		return errBad("invalid_columns")
	}
	shares, err := a.createShares(c, in.Shares)
	if err != nil {
		return err
	}
	if err := limitCount(c.ctx(), a.DB, maxPerOwner, `SELECT COUNT(*) FROM boards WHERE owner_id=?`, c.U.ID); err != nil {
		return err
	}
	id, now := crypto.NewID(), a.unix()
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `INSERT INTO boards (id,owner_id,title,description,created_at,updated_at) VALUES (?,?,?,?,?,?)`,
			id, c.U.ID, in.Title, clean(in.Description, 1000), now, now); err != nil {
			return err
		}
		for i, col := range in.Columns {
			if col = clean(col, 60); col == "" {
				return errBad("invalid_columns")
			}
			if err := tx.Exec(c.ctx(), `INSERT INTO board_columns (id,board_id,title,position) VALUES (?,?,?,?)`, crypto.NewID(), id, col, i+1); err != nil {
				return err
			}
		}
		return setShares(c.ctx(), tx, "board", id, shares)
	})
	if err != nil {
		return err
	}
	a.logAct(c, false, "board.created", "board", id, in.Title)
	return c.JSON(map[string]string{"id": id})
}

func (a *App) hBoard(c *Ctx) error {
	id := c.Param("id")
	lvl, err := a.level(c.ctx(), c.U, "board", id)
	if err != nil || lvl < LevelRead {
		return errNotFound
	}
	b := Board{Level: lvl, Columns: []Column{}, CardList: []Card{}}
	if err := a.DB.Row(c.ctx(), `SELECT id, owner_id, title, description, updated_at FROM boards WHERE id=?`, id).
		Scan(&b.ID, &b.OwnerID, &b.Title, &b.Description, &b.UpdatedAt); err != nil {
		return err
	}
	err = a.DB.Each(c.ctx(), `SELECT id, title, position FROM board_columns WHERE board_id=? ORDER BY position`, []any{id}, func(r *sql.Rows) error {
		var col Column
		if err := r.Scan(&col.ID, &col.Title, &col.Position); err != nil {
			return err
		}
		b.Columns = append(b.Columns, col)
		return nil
	})
	if err != nil {
		return err
	}
	err = a.DB.Each(c.ctx(), `SELECT id, column_id, title, description, assignee_id, due_at, position, created_by, updated_at FROM cards WHERE board_id=? ORDER BY position, id`,
		[]any{id}, func(r *sql.Rows) error {
			var k Card
			if err := r.Scan(&k.ID, &k.ColumnID, &k.Title, &k.Description, &k.AssigneeID, &k.DueAt, &k.Position, &k.CreatedBy, &k.UpdatedAt); err != nil {
				return err
			}
			b.CardList = append(b.CardList, k)
			return nil
		})
	if err != nil {
		return err
	}
	return c.JSON(b)
}

func (a *App) hUpdateBoard(c *Ctx) error {
	id := c.Param("id")
	var in struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.need(c, "board", id, LevelManage); err != nil {
		return err
	}
	if in.Title = clean(in.Title, 120); in.Title == "" {
		return errBad("title_required")
	}
	if err := a.DB.Exec(c.ctx(), `UPDATE boards SET title=?, description=?, updated_at=? WHERE id=?`, in.Title, clean(in.Description, 1000), a.unix(), id); err != nil {
		return err
	}
	a.hub.publish(Event{"board", id})
	return c.OK()
}

func (a *App) hDeleteBoard(c *Ctx) error {
	id := c.Param("id")
	if err := a.need(c, "board", id, LevelManage); err != nil {
		return err
	}
	if err := a.deleteRes(c.ctx(), "board", id); err != nil {
		return err
	}
	a.logAct(c, false, "board.deleted", "board", id, "")
	return c.OK()
}

func (a *App) hAddColumn(c *Ctx) error {
	id := c.Param("id")
	var in struct {
		Title string `json:"title"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.need(c, "board", id, LevelManage); err != nil {
		return err
	}
	if in.Title = clean(in.Title, 60); in.Title == "" {
		return errBad("title_required")
	}
	cid := crypto.NewID()
	if err := a.DB.Exec(c.ctx(), `INSERT INTO board_columns (id,board_id,title,position) VALUES (?,?,?,(SELECT COALESCE(MAX(position),0)+1 FROM board_columns WHERE board_id=?))`,
		cid, id, in.Title, id); err != nil {
		return err
	}
	a.hub.publish(Event{"board", id})
	return c.JSON(map[string]string{"id": cid})
}

func (a *App) hUpdateColumn(c *Ctx) error {
	id, cid := c.Param("id"), c.Param("col")
	var in struct {
		Title    *string `json:"title"`
		Position *int    `json:"position"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.need(c, "board", id, LevelManage); err != nil {
		return err
	}
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		var n int
		if err := tx.Row(c.ctx(), `SELECT COUNT(*) FROM board_columns WHERE id=? AND board_id=?`, cid, id).Scan(&n); err != nil || n == 0 {
			return errNotFound
		}
		if in.Title != nil && clean(*in.Title, 60) != "" {
			if err := tx.Exec(c.ctx(), `UPDATE board_columns SET title=? WHERE id=?`, clean(*in.Title, 60), cid); err != nil {
				return err
			}
		}
		if in.Position != nil {
			return reorder(c.ctx(), tx, "board_columns", "board_id", id, cid, *in.Position)
		}
		return nil
	})
	if err != nil {
		return err
	}
	a.hub.publish(Event{"board", id})
	return c.OK()
}

func (a *App) hDeleteColumn(c *Ctx) error {
	id, cid := c.Param("id"), c.Param("col")
	if err := a.need(c, "board", id, LevelManage); err != nil {
		return err
	}
	var n int
	if err := a.DB.Row(c.ctx(), `SELECT COUNT(*) FROM cards WHERE column_id=? AND board_id=?`, cid, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return errBad("column_not_empty")
	}
	if err := a.DB.Exec(c.ctx(), `DELETE FROM board_columns WHERE id=? AND board_id=?`, cid, id); err != nil {
		return err
	}
	a.hub.publish(Event{"board", id})
	return c.OK()
}

type cardIn struct {
	ColumnID    *string `json:"column_id"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	AssigneeID  *string `json:"assignee_id"`
	DueAt       *int64  `json:"due_at"`
	Position    *int    `json:"position"`
}

func (a *App) hAddCard(c *Ctx) error {
	id := c.Param("id")
	var in cardIn
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.need(c, "board", id, LevelWrite); err != nil {
		return err
	}
	if in.Title == nil || clean(*in.Title, 200) == "" || in.ColumnID == nil {
		return errBad("title_required")
	}
	if in.DueAt != nil && !validTime(*in.DueAt) {
		return errBad("invalid_time")
	}
	if err := limitCount(c.ctx(), a.DB, maxCardsBoard, `SELECT COUNT(*) FROM cards WHERE board_id=?`, id); err != nil {
		return err
	}
	kid, now := crypto.NewID(), a.unix()
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		var n, pos int
		if err := tx.Row(c.ctx(), `SELECT COUNT(*) FROM board_columns WHERE id=? AND board_id=?`, *in.ColumnID, id).Scan(&n); err != nil || n == 0 {
			return errBad("unknown_column")
		}
		if err := tx.Row(c.ctx(), `SELECT COALESCE(MAX(position),0)+1 FROM cards WHERE column_id=?`, *in.ColumnID).Scan(&pos); err != nil {
			return err
		}
		desc, assignee, due := "", "", int64(0)
		if in.Description != nil {
			desc = clean(*in.Description, 4000)
		}
		if in.AssigneeID != nil {
			assignee = *in.AssigneeID
		}
		if in.DueAt != nil {
			due = *in.DueAt
		}
		if err := tx.Exec(c.ctx(), `INSERT INTO cards (id,board_id,column_id,title,description,assignee_id,due_at,position,created_by,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`, kid, id, *in.ColumnID, clean(*in.Title, 200), desc, clean(assignee, 40), due, pos, c.U.ID, now, now); err != nil {
			return err
		}
		return tx.Exec(c.ctx(), `UPDATE boards SET updated_at=? WHERE id=?`, now, id)
	})
	if err != nil {
		return err
	}
	a.logAct(c, false, "card.created", "board", id, *in.Title)
	a.hub.publish(Event{"board", id})
	return c.JSON(map[string]string{"id": kid})
}

func (a *App) hUpdateCard(c *Ctx) error {
	id, kid := c.Param("id"), c.Param("card")
	var in cardIn
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.need(c, "board", id, LevelWrite); err != nil {
		return err
	}
	now := a.unix()
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		ctx := c.ctx()
		var col string
		if err := tx.Row(ctx, `SELECT column_id FROM cards WHERE id=? AND board_id=?`, kid, id).Scan(&col); err != nil {
			return errNotFound
		}
		set := func(field string, v any) error {
			return tx.Exec(ctx, `UPDATE cards SET `+field+`=?, updated_at=? WHERE id=?`, v, now, kid)
		}
		if in.Title != nil && clean(*in.Title, 200) != "" {
			if err := set("title", clean(*in.Title, 200)); err != nil {
				return err
			}
		}
		if in.Description != nil {
			if err := set("description", clean(*in.Description, 4000)); err != nil {
				return err
			}
		}
		if in.AssigneeID != nil {
			if err := set("assignee_id", clean(*in.AssigneeID, 40)); err != nil {
				return err
			}
		}
		if in.DueAt != nil {
			if !validTime(*in.DueAt) {
				return errBad("invalid_time")
			}
			if err := set("due_at", *in.DueAt); err != nil {
				return err
			}
		}
		if in.ColumnID != nil && *in.ColumnID != col {
			var n int
			if err := tx.Row(ctx, `SELECT COUNT(*) FROM board_columns WHERE id=? AND board_id=?`, *in.ColumnID, id).Scan(&n); err != nil || n == 0 {
				return errBad("unknown_column")
			}
			col = *in.ColumnID
			if err := set("column_id", col); err != nil {
				return err
			}
			if in.Position == nil {
				end := 1 << 30
				in.Position = &end
			}
		}
		if in.Position != nil {
			if err := reorder(ctx, tx, "cards", "column_id", col, kid, *in.Position); err != nil {
				return err
			}
		}
		return tx.Exec(ctx, `UPDATE boards SET updated_at=? WHERE id=?`, now, id)
	})
	if err != nil {
		return err
	}
	a.hub.publish(Event{"board", id})
	return c.OK()
}

func (a *App) hDeleteCard(c *Ctx) error {
	id, kid := c.Param("id"), c.Param("card")
	if err := a.need(c, "board", id, LevelWrite); err != nil {
		return err
	}
	if err := a.DB.Exec(c.ctx(), `DELETE FROM cards WHERE id=? AND board_id=?`, kid, id); err != nil {
		return err
	}
	a.hub.publish(Event{"board", id})
	return c.OK()
}
