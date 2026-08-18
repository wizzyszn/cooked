package domain

// User is an account that authors recipes and may contribute delicacies.
type User struct {
	BaseModel
	Email      string `gorm:"size:255;not null;uniqueIndex" json:"email"`
	Name       string `gorm:"size:255;not null" json:"name"`
	UserName   string `gorm:"size:24;not null" json:"user_name"`
	Picture    string `gorm:"size:512" json:"picture,omitempty"`
	IsVerified bool   `gorm:"not null;default:false" json:"is_verified"`
	HashPass   string `gorm:"size:255" json:"-"`

	// Content
	Recipes    []Recipe   `gorm:"foreignKey:UserID" json:"recipes,omitempty"`
	Delicacies []Delicacy `gorm:"foreignKey:CreatedBy" json:"delicacies,omitempty"`

	// Social
	Favorites []Favorite `gorm:"foreignKey:UserID" json:"favorites,omitempty"`
	Ratings   []Rating   `gorm:"foreignKey:UserID" json:"ratings,omitempty"`
	Comments  []Comment  `gorm:"foreignKey:UserID" json:"comments,omitempty"`
	Following []Follow   `gorm:"foreignKey:FollowerID" json:"following,omitempty"`
	Followers []Follow   `gorm:"foreignKey:FollowingID" json:"followers,omitempty"`
}

func (User) TableName() string { return "users" }

type SanitizedUser struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	UserName   string `json:"user_name"`
	IsVerified bool   `json:"is_verified"`
	Picture    string `json:"picture,omitempty"`
}

func (u *User) Sanitize() *SanitizedUser {
	if u == nil {
		return nil
	}
	return &SanitizedUser{
		ID:         u.ID.String(),
		Email:      u.Email,
		Name:       u.Name,
		UserName:   u.UserName,
		IsVerified: u.IsVerified,
		Picture:    u.Picture,
	}
}
