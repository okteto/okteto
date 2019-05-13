const initialSpacesState = {
  list: [],
  deleting: [],
  current: null,
  currentId: null,
  isFetching: false,
  isLoaded: false
};

const sortSpaces = spaces => {
  const [home, ...rest] = spaces;
  return [home, ...rest.sort((a, b) => {
    var nameA = a.name.toUpperCase();
    var nameB = b.name.toUpperCase();
    if (nameA < nameB) return -1;
    if (nameA > nameB) return 1;
    return 0;
  })];
};

export default (state = initialSpacesState, action) => {
  switch (action.type) {
    case 'REQUEST_SPACES': {
      return {
        ...state, 
        isFetching: true
      };
    }
    case 'RECEIVE_SPACES': {
      return {
        ...state,
        list: sortSpaces(action.spaces),
        isFetching: false
      };
    }
    case 'REQUEST_SPACE': {
      return {
        ...state,
        isFetching: true
      };
    }
    case 'RECEIVE_SPACE': {
      return {
        ...state,
        current: action.space,
        isFetching: false,
        isLoaded: true
      };
    }
    case 'DISCARD_RECEIVE_SPACE': {
      return {
        ...state,
        isFetching: false,
      };
    }
    case 'CHANGE_CURRENT_SPACE': {
      return {
        ...state,
        currentId: action.spaceId,
      };
    }
    case 'FAILED_RECEIVE_SPACES': 
    case 'FAILED_RECEIVE_SPACE': {
      return state;
    }
    case 'DELETING_SPACE': {
      return {
        ...state,
        deleting: [...state.deleting, action.spaceId]
      };
    }
    case 'DELETED_SPACE': {
      return {
        ...state,
        deleting: state.deleting.filter(id => id !== action.spaceId)
      };
    }
    default: return state;
  }
};
