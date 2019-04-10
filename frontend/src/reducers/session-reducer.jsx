const initialSessionState = {
  user: {},
  isAuthenticated: true, // false // Change once Authentication is done.
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
    default: {
      return state;
    }
  }
};