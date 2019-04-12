import { SESSION_KEY } from 'actions/session';

const initialSessionState = {
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
    case 'SAVE_SESSION': {
      localStorage.setItem(SESSION_KEY, JSON.stringify(state || initialSessionState));
      return state;
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
    case 'RESTORE_SESSION_SUCCESS': {
      return action.session;
    }
    case 'LOGOUT': {
      return {...initialSessionState};
    }
    default: {
      return state;
    }
  }
};