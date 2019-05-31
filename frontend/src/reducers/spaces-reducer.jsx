const initialSpacesState = {
  list: [],
  deleting: [],
  current: null,
  currentId: null,
  isFetching: false,
  isLoaded: false
};

const allResourcesLoaded = state => {
  return !!state.current && state.list.length > 0;
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
      const newState = {
        ...state,
        list: action.spaces,
        isFetching: false
      };
      
      return {
        ...newState,
        isLoaded: allResourcesLoaded(newState)
      };
    }
    case 'REQUEST_SPACE': {
      return {
        ...state,
        isFetching: true
      };
    }
    case 'RECEIVE_SPACE': {
      const newState = {
        ...state,
        current: action.space,
        isFetching: false
      };

      return {
        ...newState,
        isLoaded: allResourcesLoaded(newState)
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
