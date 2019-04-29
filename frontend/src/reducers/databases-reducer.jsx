const initialDatabasesState = {
  byName: {},
  isFetching: false
};

export default (state = initialDatabasesState, action) => {
  switch (action.type) {
    case 'REQUEST_DATABASES': {
      return {
        ...state, 
        isFetching: true
      };
    }
    case 'RECEIVE_DATABASES': {
      return {
        ...state,
        byName: Array.from(action.databases).reduce((map, database) => {
          map[database.name] = database;
          return map;
        }, {}),
        isFetching: false
      };
    }
    case 'FAILED_RECEIVE_DATABASES': {
      return {
        ...initialDatabasesState
      }
    }
    default: return state;
  }
};
