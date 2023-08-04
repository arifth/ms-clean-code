package repository

import (
	"bitbucket.org/bridce/ms-clean-code/infras/database"
	"bitbucket.org/bridce/ms-clean-code/internal/domain/user/model"
	"bitbucket.org/bridce/ms-clean-code/internal/domain/user/query"
	"errors"
	"fmt"
	"strings"
)

type UserRepoStrct struct {
	db *database.Conn
}

func UserRepoImpl(db *database.Conn) UserRepoInterface {
	return &UserRepoStrct{
		db: db,
	}
}

func (ur UserRepoStrct) InsertDataUser(u model.User) (*model.User, error) {
	tx := ur.db.Write.Begin()

	err := tx.Debug().Create(&u).Error
	if err != nil {
		tx.Rollback()
		return &model.User{}, err
	}

	tx.Commit()
	return &u, nil
}

func (ur UserRepoStrct) List(filter model.Filter) (result []model.ListUser, err error) {
	var (
		user        model.ListUser
		whereClause string
	)
	clauses, args, err := composeFilter(filter)
	if err != nil {
		return
	}

	baseQuery := fmt.Sprintf("%s %s",
		query.UserQuery.SelectListUser,
		whereClause,
	)

	if len(args) > 0 && clauses != "" {
		whereClause += clauses
	}
	rows, err := ur.db.Read.Debug().Raw(baseQuery+whereClause, args...).Rows()
	if err != nil {
		return
	}

	defer rows.Close()
	for rows.Next() {
		rows.Scan(&user.Nama, &user.Alamat, &user.Pendidikan, &user.FilterCount)
		result = append(result, user)
	}

	return
}

func composeFilter(filter model.Filter) (clauseStr string, args []interface{}, err error) {
	args = make([]interface{}, 0)
	clause := make([]string, 0)

	if len(filter.FilterFields) > 0 {
		var (
			whereQueries []string
			whereArgs    []interface{}
		)
		for _, filterField := range filter.FilterFields {
			switch filterField.Operator {
			case model.OperatorEqual:
				whereQueries = append(whereQueries, fmt.Sprintf("%s = ?", filterField.Field))
				whereArgs = append(whereArgs, filterField.Value)
			case model.OperatorOr:
				whereQueries = append(whereQueries, fmt.Sprintf("(%s = ? OR %s = ?)", strings.Split(filterField.Field, ",")[0], strings.Split(filterField.Field, ",")[1]))
				valueArray, ok := filterField.Value.([]interface{})
				if !ok && len(valueArray) != 2 {
					err = errors.New(fmt.Sprintf("invalid value type for operator %s", filterField.Operator))
					return
				}
				whereArgs = append(whereArgs, valueArray...)
			case model.OperatorRange:
				whereQueries = append(whereQueries, fmt.Sprintf("%s BETWEEN ? AND ?", filterField.Field))
				valueArray, ok := filterField.Value.([]interface{})
				if !ok && len(valueArray) != 2 {
					err = errors.New(fmt.Sprintf("invalid value type for operator %s", filterField.Operator))
					return
				}
				whereArgs = append(whereArgs, valueArray...)
			case model.OperatorIn:
				valueArray, ok := filterField.Value.([]interface{})
				if !ok {
					err = errors.New(fmt.Sprintf("invalid value type for operator %s", filterField.Operator))
					return
				}
				var placeholder []string
				for range valueArray {
					placeholder = append(placeholder, "?")
				}
				whereQueries = append(whereQueries, fmt.Sprintf("%s IN (%s)", filterField.Field, strings.Join(placeholder, ",")))
				whereArgs = append(whereArgs, valueArray...)
			case model.OperatorIsNull:
				value, ok := filterField.Value.(bool)
				if !ok {
					err = errors.New(fmt.Sprintf("invalid value type for operator %s", filterField.Operator))
					return
				}
				if value {
					whereQueries = append(whereQueries, fmt.Sprintf("%s IS NULL", filterField.Field))
				} else {
					whereQueries = append(whereQueries, fmt.Sprintf("%s IS NOT NULL", filterField.Field))
				}
			case model.OperatorNot:
				whereQueries = append(whereQueries, fmt.Sprintf("%s != ?", filterField.Field))
				whereArgs = append(whereArgs, filterField.Value)
			}
		}

		clause = append(clause, fmt.Sprintf(" WHERE %s", strings.Join(whereQueries, " AND ")))
		args = append(args, whereArgs...)
	}

	if len(filter.FilterFields) > 0 {
		sortQuery := []string{}
		query := ""
		cond := ""
		for _, sort := range filter.Sorts {
			if sort.Condition != "" {
				sortQuery = append(sortQuery, fmt.Sprintf("%s", sort.Field))
				cond = sort.Condition
			} else {
				sortQuery = append(sortQuery, fmt.Sprintf("%s %s", sort.Field, sort.Order))
			}
		}
		if cond == model.ConditionIfNull {
			query = fmt.Sprintf(" ORDER BY ifnull(%s) %s", strings.Join(sortQuery, ","), filter.Sorts[0].Order)
		} else {
			query = fmt.Sprintf(" ORDER BY %s", strings.Join(sortQuery, ","))
		}

		clause = append(clause, query)
	}

	if filter.Pagination.PageSize > 0 {
		query := ""
		query += fmt.Sprintf(" LIMIT %d", filter.Pagination.PageSize)
		if filter.Pagination.Page > 0 {
			query += fmt.Sprintf(" OFFSET %d", (filter.Pagination.Page-1)*filter.Pagination.PageSize)
		}
		clause = append(clause, query)
	}

	clauseStr = strings.Join(clause, "")

	return
}
