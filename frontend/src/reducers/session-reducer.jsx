const initialSessionState = {
  // user: {
  //   username: 'cindy',
  //   email: 'cindy@okteto.com'
  // },
  // isAuthenticated: true, // false // Change once Authentication is done.
  user: {},
  isAuthenticated: false,
};

export default (state = initialSessionState, action) => {
  switch (action.type) {
    case 'AUTH_SUCCESS': {
      return action.user ? {
        user: action.user,
        isAuthenticated: true
      } : {
        ...initialSessionState
      };
    }
    case 'LOGOUT': {
      return {...initialSessionState};
    }
    case 'UPDATE_SESSION': {
      return {
        ...state,
        user: {
          ...state.user,
          ...action.user
        }
      };
    }
    case 'SAVE_SESSION': {
      localStorage.setItem('session', JSON.stringify(state || {}));
      return state;
    }
    default: {
      return state;
    }
  }
};